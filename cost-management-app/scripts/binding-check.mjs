import { readFile } from 'node:fs/promises';
import { classifyLiveBindings } from '../src/live-bindings.mjs';

// Typed live-binding classification. Expected not-ready states are successful
// classifications printed to stdout at exit 0 ({verdict, reason}). Exit is
// non-zero only when the file exists but cannot be read or parsed.
function emit(result, exitCode = 0) {
  const line = JSON.stringify(result, null, 2);
  if (exitCode === 0) {
    console.log(line);
  } else {
    console.error(line);
  }
  process.exit(exitCode);
}

let raw;
try {
  raw = await readFile('data/live-bindings.json', 'utf8');
} catch (error) {
  if (error && error.code === 'ENOENT') {
    emit({
      ...classifyLiveBindings(null),
      requiredFile: 'data/live-bindings.json',
      exampleFile: 'data/live-bindings.example.json',
      message: 'No deployment-local live bindings yet. This is the correct safe default on a fresh clone.',
    });
  }
  emit({verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNREADABLE', status: 'LIVE_BINDINGS_UNREADABLE', detail: String((error && error.message) || error)}, 1);
}

let bindings;
try {
  bindings = JSON.parse(raw);
} catch (error) {
  emit({verdict: 'ERROR', reason: 'LIVE_BINDINGS_UNPARSEABLE', status: 'LIVE_BINDINGS_UNPARSEABLE', detail: String((error && error.message) || error)}, 1);
}

emit(classifyLiveBindings(bindings));
