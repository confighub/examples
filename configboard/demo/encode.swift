// Encodes a manifest of PNG frames + burned-in captions into a silent H.264 .mp4
// using only AVFoundation/AppKit, so it needs no ffmpeg or ImageMagick.
//
//   swift encode.swift <manifest.tsv> <out.mp4>
//
// Manifest lines are: <png filename>\t<seconds>\t<caption, \n for a line break>

import AVFoundation
import AppKit

let args = CommandLine.arguments
guard args.count == 3 else {
    FileHandle.standardError.write("usage: encode.swift <manifest.tsv> <out.mp4>\n".data(using: .utf8)!)
    exit(2)
}
let manifestURL = URL(fileURLWithPath: args[1])
let outURL = URL(fileURLWithPath: args[2])
let dir = manifestURL.deletingLastPathComponent()

struct Item {
    let url: URL
    let seconds: Double
    let caption: String
}

let text = try String(contentsOf: manifestURL, encoding: .utf8)
var items: [Item] = []
for line in text.split(separator: "\n", omittingEmptySubsequences: true) {
    let parts = line.split(separator: "\t", maxSplits: 2, omittingEmptySubsequences: false)
    guard parts.count == 3, let secs = Double(parts[1]) else { continue }
    items.append(Item(
        url: dir.appendingPathComponent(String(parts[0])),
        seconds: secs,
        caption: String(parts[2]).replacingOccurrences(of: "\\n", with: "\n")
    ))
}
guard !items.isEmpty else {
    FileHandle.standardError.write("no frames in manifest\n".data(using: .utf8)!)
    exit(1)
}

// Layout. The source screenshots are 3000x1882 (2x retina); the video is half that
// width with a caption band underneath. Both dimensions must stay even for H.264.
let W = 1920
let imageH = 1204
let bandH = 146
let H = imageH + bandH

let fps: Int32 = 10
let timescale: Int32 = 600

let bg = NSColor(calibratedRed: 0.055, green: 0.063, blue: 0.078, alpha: 1)
let captionInk = NSColor(calibratedWhite: 0.94, alpha: 1)
let counterInk = NSColor(calibratedWhite: 0.55, alpha: 1)

try? FileManager.default.removeItem(at: outURL)
let writer = try AVAssetWriter(outputURL: outURL, fileType: .mp4)
let settings: [String: Any] = [
    AVVideoCodecKey: AVVideoCodecType.h264,
    AVVideoWidthKey: W,
    AVVideoHeightKey: H,
    AVVideoCompressionPropertiesKey: [
        AVVideoAverageBitRateKey: 3_000_000,
        AVVideoMaxKeyFrameIntervalKey: fps * 10,
        AVVideoProfileLevelKey: AVVideoProfileLevelH264HighAutoLevel,
    ],
]
let input = AVAssetWriterInput(mediaType: .video, outputSettings: settings)
input.expectsMediaDataInRealTime = false
let adaptor = AVAssetWriterInputPixelBufferAdaptor(
    assetWriterInput: input,
    sourcePixelBufferAttributes: [
        kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
        kCVPixelBufferWidthKey as String: W,
        kCVPixelBufferHeightKey as String: H,
    ]
)
writer.add(input)
writer.startWriting()
writer.startSession(atSourceTime: .zero)

var pool: CVPixelBufferPool?
let poolAttrs: [String: Any] = [
    kCVPixelBufferPixelFormatTypeKey as String: Int(kCVPixelFormatType_32BGRA),
    kCVPixelBufferWidthKey as String: W,
    kCVPixelBufferHeightKey as String: H,
    kCVPixelBufferCGImageCompatibilityKey as String: true,
]
CVPixelBufferPoolCreate(kCFAllocatorDefault, nil, poolAttrs as CFDictionary, &pool)

func render(_ item: Item, index: Int, total: Int) -> CVPixelBuffer? {
    guard let pool = pool else { return nil }
    var pbOut: CVPixelBuffer?
    guard CVPixelBufferPoolCreatePixelBuffer(kCFAllocatorDefault, pool, &pbOut) == kCVReturnSuccess,
          let pb = pbOut else { return nil }

    CVPixelBufferLockBaseAddress(pb, [])
    defer { CVPixelBufferUnlockBaseAddress(pb, []) }

    guard let base = CVPixelBufferGetBaseAddress(pb) else { return nil }
    let space = CGColorSpaceCreateDeviceRGB()
    guard let ctx = CGContext(
        data: base,
        width: W,
        height: H,
        bitsPerComponent: 8,
        bytesPerRow: CVPixelBufferGetBytesPerRow(pb),
        space: space,
        bitmapInfo: CGImageAlphaInfo.premultipliedFirst.rawValue | CGBitmapInfo.byteOrder32Little.rawValue
    ) else { return nil }

    ctx.setFillColor(bg.cgColor)
    ctx.fill(CGRect(x: 0, y: 0, width: W, height: H))

    // Screenshot on top (CG origin is bottom-left, so it sits above the band).
    if let img = NSImage(contentsOf: item.url),
       let cg = img.cgImage(forProposedRect: nil, context: nil, hints: nil) {
        ctx.interpolationQuality = .high
        ctx.draw(cg, in: CGRect(x: 0, y: CGFloat(bandH), width: CGFloat(W), height: CGFloat(imageH)))
    }

    // Caption band.
    let gc = NSGraphicsContext(cgContext: ctx, flipped: false)
    NSGraphicsContext.saveGraphicsState()
    NSGraphicsContext.current = gc

    let para = NSMutableParagraphStyle()
    para.lineSpacing = 6
    let caption = NSAttributedString(string: item.caption, attributes: [
        .font: NSFont.systemFont(ofSize: 27, weight: .regular),
        .foregroundColor: captionInk,
        .paragraphStyle: para,
    ])
    caption.draw(in: CGRect(x: 44, y: 22, width: CGFloat(W) - 220, height: CGFloat(bandH) - 40))

    let counter = NSAttributedString(string: "\(index + 1) / \(total)", attributes: [
        .font: NSFont.monospacedDigitSystemFont(ofSize: 22, weight: .regular),
        .foregroundColor: counterInk,
    ])
    let cs = counter.size()
    counter.draw(at: NSPoint(x: CGFloat(W) - 44 - cs.width, y: CGFloat(bandH) - 26 - cs.height))

    NSGraphicsContext.restoreGraphicsState()
    return pb
}

var frameIndex: Int64 = 0
for (i, item) in items.enumerated() {
    guard let pb = render(item, index: i, total: items.count) else {
        FileHandle.standardError.write("failed to render \(item.url.lastPathComponent)\n".data(using: .utf8)!)
        exit(1)
    }
    let repeats = max(1, Int((item.seconds * Double(fps)).rounded()))
    for _ in 0..<repeats {
        while !input.isReadyForMoreMediaData { usleep(2000) }
        let pts = CMTime(value: frameIndex * Int64(timescale / fps), timescale: timescale)
        if !adaptor.append(pb, withPresentationTime: pts) {
            FileHandle.standardError.write("append failed: \(writer.error?.localizedDescription ?? "?")\n".data(using: .utf8)!)
            exit(1)
        }
        frameIndex += 1
    }
    FileHandle.standardError.write("  \(i + 1)/\(items.count) \(item.url.lastPathComponent)\n".data(using: .utf8)!)
}

input.markAsFinished()
let done = DispatchSemaphore(value: 0)
writer.finishWriting { done.signal() }
done.wait()

if writer.status == .completed {
    let secs = Double(frameIndex) / Double(fps)
    print(String(format: "wrote %@ (%.1fs, %d frames)", outURL.path, secs, frameIndex))
} else {
    FileHandle.standardError.write("writer failed: \(writer.error?.localizedDescription ?? "?")\n".data(using: .utf8)!)
    exit(1)
}
