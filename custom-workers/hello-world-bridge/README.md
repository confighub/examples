# Custom Bridge Example

This guide explains how to create custom bridges for ConfigHub. Bridges are adapters that connect ConfigHub to external systems, allowing you to apply, refresh, import, and destroy configuration resources on various targets.

## Quick Start

Log into ConfigHub with the CLI:

    cub auth login

Build the example in this directory:

    go build

### Running locally with `cub worker run`

The simplest way to run the example is with `cub worker run`, which automatically creates the worker and sets up the environment:

    cub worker run --space $SPACE --executable ./hello-world-bridge hello-bridge

This will create the worker if it doesn't exist, set the required environment variables (`CONFIGHUB_WORKER_ID`, `CONFIGHUB_WORKER_SECRET`, `CONFIGHUB_URL`), and start the executable.

### Running directly with environment variables

Alternatively, you can set up the environment manually:

    eval "$(cub worker get-envs --space $SPACE hello-bridge)"
    export EXAMPLE_BRIDGE_DIR=/tmp/confighub-example-bridge  # Optional: defaults to /tmp/confighub-example-bridge
    ./hello-world-bridge

### Installing in a Kubernetes cluster

To deploy the worker in a Kubernetes cluster, first build and push a container image:

    docker build -f Dockerfile -t my-registry/hello-world-bridge:latest .
    docker push my-registry/hello-world-bridge:latest

Then install using `cub worker install`:

    # Create the worker unit in ConfigHub
    cub worker install --space $SPACE \
      --unit hello-bridge-unit \
      --target $TARGET \
      --image my-registry/hello-world-bridge:latest \
      hello-bridge

    # Apply the worker unit to the cluster
    cub unit apply --space $SPACE hello-bridge-unit

    # Wait for the namespace and deployment, then install the secret
    kubectl -n confighub wait --for=create deployment/hello-bridge --timeout=120s
    cub worker install --space $SPACE \
      --export-secret-only \
      -n confighub \
      hello-bridge 2>/dev/null | kubectl apply -f -

    # Wait for the worker to be ready
    kubectl -n confighub rollout status deployment/hello-bridge --timeout=120s

### Verifying the worker

Create the base directory and a test subdirectory (which will become a target):

    mkdir -p /tmp/confighub-example-bridge/dev

It should connect to ConfigHub and display:

    [INFO] Starting hello-world-bridge example...
    [INFO] Using base directory: /tmp/confighub-example-bridge
    [INFO] Starting connector...

Note: You may see warnings about uninitialized loggers - these can be safely ignored.

Create a unit with some Kubernetes compliant YAML content:

    cub unit create myapp test_input.yaml --target hello-bridge-filesystem-kubernetes-yaml-dev

Apply the unit to your bridge target:

    cub unit apply myapp

The bridge will write the configuration to `/tmp/confighub-example-bridge/dev/myapp.yaml`.

## Bridge Operations

Bridges implement five core operations. An operation is always performed on a config Unit. The Unit must be associated with a Target made available by the bridge, and the Unit's ToolchainType must match one of the Target's ConfigTypes.

### 1. Apply

Applies configuration to the target system. The bridge receives the Unit's Data (desired configuration) and should write/deploy it to the target. The bridge should return updated LiveData and LiveState on completion. In this example, Apply writes files on the filesystem.

    cub unit apply myapp

### 2. Refresh

Reads the current state from the target and detects drift. The bridge compares the live state with the expected Data and reports whether drift was detected. It should return updated LiveData on completion.

    cub unit refresh myapp

### 3. Import

Discovers existing resources in the target system and imports them as Data. In this example, it reads a file from the target directory.

### 4. Destroy

Removes configuration from the target system. In this example, it deletes files. The bridge should return empty LiveData after destruction.

    cub unit destroy myapp

### 5. Finalize

Performs cleanup operations after other actions. Implementation-specific.

## Core Concepts

### BridgeWorker Interface

Every bridge must implement the `BridgeWorker` interface (aliased as `Bridge`):

```go
type BridgeWorker interface {
    ID() BridgeWorkerID
    Info(InfoOptions) BridgeWorkerInfo
    Apply(BridgeWorkerContext, BridgeWorkerPayload) error
    Refresh(BridgeWorkerContext, BridgeWorkerPayload) error
    Import(BridgeWorkerContext, BridgeWorkerPayload) error
    Destroy(BridgeWorkerContext, BridgeWorkerPayload) error
    Finalize(BridgeWorkerContext, BridgeWorkerPayload) error
}
```

The SDK also provides shorter aliases: `Bridge` for `BridgeWorker`, `BridgeContext` for `BridgeWorkerContext`, `BridgePayload` for `BridgeWorkerPayload`, and `BridgeInfo` for `BridgeWorkerInfo`.

### Bridge Identification

The `ID()` method returns a `BridgeWorkerID` that identifies the bridge by its `ProviderType` and the `ToolchainTypes` it supports:

```go
func (eb *ExampleBridge) ID() api.BridgeWorkerID {
    return api.BridgeWorkerID{
        ProviderType:   ProviderFilesystem,
        ToolchainTypes: []workerapi.ToolchainType{workerapi.ToolchainKubernetesYAML},
    }
}
```

The `ProviderType` identifies a particular bridge implementation. Each bridge should define a unique `ProviderType` that the worker's dispatcher uses to route operations to the correct bridge. It is best practice for the `ProviderType` to be globally unique, but this is not strictly required.

### Capabilities and ConfigTypes

The `Info()` method returns a `BridgeWorkerInfo` describing the bridge's capabilities. This includes the `SupportedConfigTypes` and `AvailableTargets` discovered by the bridge.

Each `SupportedConfigType` contains a `ConfigTypeSignature` with:

- **ProviderType** - Identifies this bridge implementation (e.g., `"Filesystem"`, `"Kubernetes"`, `"ConfigMapRenderer"`)
- **ToolchainType** - The configuration toolchain/format of the Data (e.g., `"Kubernetes/YAML"`, `"AppConfig/YAML"`, `"AppConfig/TOML"`)
- **LiveStateType** (optional) - The toolchain/format of the LiveState (e.g., `"Kubernetes/YAML"`). Required in order to invoke functions on LiveState.
- **Options** (optional) - A list of `BridgeOption` definitions supported by this ConfigType

```go
func (eb *ExampleBridge) Info(opts api.InfoOptions) api.BridgeInfo {
    // Discover targets by scanning subdirectories
    var targets []api.Target
    entries, _ := os.ReadDir(eb.baseDir)
    for _, entry := range entries {
        if entry.IsDir() {
            targets = append(targets, api.Target{
                BridgeHandle: entry.Name(),
                Name:         api.GenerateTargetName(opts.WorkerSlug, ProviderFilesystem, workerapi.ToolchainKubernetesYAML, entry.Name()),
            })
        }
    }

    return api.BridgeInfo{
        SupportedConfigTypes: []*api.SupportedConfigType{
            {
                ConfigTypeSignature: api.ConfigTypeSignature{
                    ConfigType: api.ConfigType{
                        ToolchainType: workerapi.ToolchainKubernetesYAML,
                        ProviderType:  ProviderFilesystem,
                    },
                    Options: []api.BridgeOption{
                        {
                            Name:        "SubDir",
                            Description: "Optional subdirectory within the BridgeHandle directory",
                            Required:    false,
                            DataType:    funcapi.DataTypeString,
                            Example:     "app1",
                        },
                    },
                },
                AvailableTargets: targets,
            },
        },
    }
}
```

### Bridge Options

Bridges can define `BridgeOption` entries in their `ConfigTypeSignature` to declare configurable parameters. Each option has:

- **Name** - Option name in PascalCase (e.g., `"DirName"`, `"FluxNamespace"`)
- **Description** - Human-readable description
- **Required** - Whether the option must be provided
- **DataType** - The data type of the option value (e.g., string, int, bool)
- **Example** (optional) - An example value

Options are set on Target entities (and optionally per-Unit via `TargetOptions`). They are validated against the BridgeWorker's defined Options for the matching ProviderType+ToolchainType+LiveStateType signature. When an operation is dispatched, the bridge receives the merged option values in `BridgeWorkerPayload.TargetOptions`.

In this example, the `BridgeHandle` identifies the top-level target directory (e.g., `"dev"`), and an optional `SubDir` option allows targeting a subdirectory within it:

```go
func parseTargetDir(payload api.BridgeWorkerPayload) string {
    dir := payload.BridgeHandle
    if dir == "" {
        dir = "default"
    }
    if subDir, ok := payload.TargetOptions["SubDir"]; ok && subDir != "" {
        dir = filepath.Join(dir, subDir)
    }
    return dir
}
```

### Target Discovery

The bridge may discover multiple systems under management of the same ConfigType signature, each with their own context (including credentials and coordinates). These are published as `AvailableTargets` for each `SupportedConfigType`.

Each `AvailableTarget` has:

- **BridgeHandle** - A unique identifier within the bridge implementation for looking up the appropriate context (credentials, coordinates, etc.) when processing operations. The worker should be able to map a BridgeHandle back to the correct context.
- **Name** (optional) - If non-empty, ConfigHub will attempt to automatically create a Target entity with that slug in the same Space as the worker.

In this example, each subdirectory is a target, and the directory name serves as the BridgeHandle:

```go
api.Target{
    BridgeHandle: entry.Name(),
    Name:         api.GenerateTargetName(opts.WorkerSlug, ProviderFilesystem, workerapi.ToolchainKubernetesYAML, entry.Name()),
}
```

### Targets in ConfigHub

A Target entity created in ConfigHub supports one or more ConfigTypes, each with ProviderType, ToolchainType, LiveStateType, and optional Options. These are validated against the ConfigTypes supported by the corresponding BridgeWorker. A Unit's ToolchainType and optional ProviderType are validated against the attached Target's ConfigTypes.

### Status Reporting

Bridges report operation progress using `SendStatus()`. For example:

```go
startTime := time.Now()
if err := ctx.SendStatus(&api.ActionResult{
    UnitID:            payload.UnitID,
    SpaceID:           payload.SpaceID,
    QueuedOperationID: payload.QueuedOperationID,
    ActionResultBaseMeta: api.ActionResultBaseMeta{
        Action:    api.ActionApply,
        Result:    api.ActionResultNone,
        Status:    api.ActionStatusProgressing,
        Message:   "Starting apply operation",
        StartedAt: startTime,
    },
}); err != nil {
    return err
}

// ... perform operation ...

terminatedAt := time.Now()
return ctx.SendStatus(&api.ActionResult{
    UnitID:            payload.UnitID,
    SpaceID:           payload.SpaceID,
    QueuedOperationID: payload.QueuedOperationID,
    ActionResultBaseMeta: api.ActionResultBaseMeta{
        Action:       api.ActionApply,
        Result:       api.ActionResultApplyCompleted,
        Status:       api.ActionStatusCompleted,
        Message:      "Successfully applied configuration",
        StartedAt:    startTime,
        TerminatedAt: &terminatedAt,
    },
    Data:     payload.Data,
    LiveData: payload.Data,
})
```

### Drift Detection

The Refresh operation compares the live state in the target system with the expected configuration Data. In this example, it does a byte comparison between the Unit's Data and the file contents. The bridge reports `ActionResultRefreshAndDrifted` or `ActionResultRefreshAndNoDrift` accordingly.

## Bridge Registration

Register your bridge with the dispatcher and pass the dispatcher to the Connector:

```go
bridgeDispatcher := worker.NewBridgeDispatcher()
bridgeDispatcher.RegisterBridge(NewExampleBridge("example-bridge", baseDir))

connector, err := worker.NewConnector(worker.ConnectorOptions{
    WorkerID:         os.Getenv("CONFIGHUB_WORKER_ID"),
    WorkerSecret:     os.Getenv("CONFIGHUB_WORKER_SECRET"),
    ConfigHubURL:     os.Getenv("CONFIGHUB_URL"),
    BridgeDispatcher: &bridgeDispatcher,
})
```

## Data Types

### BridgeWorkerPayload

Contains all information about the operation dispatched to the bridge:

- `QueuedOperationID` - Links this operation back to the original request
- `ToolchainType` - Configuration toolchain/format of the Unit (e.g., `"Kubernetes/YAML"`)
- `ProviderType` - ProviderType of the Target attached to the Unit
- `BridgeHandle` - Identifier for the Target's credentials and coordinates within this bridge
- `UnitID`, `SpaceID`, `OrganizationID` - Entity identifiers
- `UnitSlug`, `SpaceSlug` - Human-readable slugs
- `UnitLabels`, `UnitAnnotations`, `SpaceLabels`, `SpaceAnnotations` - Metadata
- `TargetID` - UUID of the Target attached to the Unit
- `TargetOptions` - Merged bridge option values from the Target and Unit for the matching ConfigType
- `Data` - Current configuration data for the Unit. This is what Apply, Destroy, Refresh, and Import primarily operate on.
- `LiveData` - Live resources as of the most recent action, in the same representation as Data (i.e., same ToolchainType). May be used for drift detection in comparisons with Data.
- `LiveState` - Live state as of the most recent action. The content is ProviderType-specific, with the toolchain/format identified by the ConfigType's LiveStateType. Functions may operate on LiveState as well as on Data.
- `BridgeState` - Additional ProviderType-specific state used by the bridge. Currently considered internal to the bridge and does not have a defined toolchain/format.
- `DryRun` - Whether the action is a dry run
- `RevisionNum`, `LiveRevisionNum` - Sequence numbers for revision tracking
- `DriftReconciliationMode` - The drift reconciliation mode for the unit

### ActionResult

Reports operation status and results back to ConfigHub:

- `UnitID`, `SpaceID`, `QueuedOperationID` - Entity identifiers
- `ActionResultBaseMeta` - Action type, result, status, message, timing
- `Data` - Updated configuration data (e.g., for Import)
- `LiveData` - The bridge should return updated LiveData on all operations, representing the live resources in the same format as Data (ToolchainType)
- `LiveState` - The bridge should return updated LiveState on all operations. The content is ProviderType-specific (format identified by LiveStateType)
- `BridgeState` - Additional ProviderType-specific state used by the bridge
- `ResourceStatuses` - Per-resource sync and readiness status
- `ErrorMessages` - Warning or error messages to surface to the user

## Best Practices

### Error Handling

Always report errors with appropriate status:

```go
if err != nil {
    return fmt.Errorf("failed to write file %s: %w", filepath, err)
}
```

For more complex error handling, you can send status updates before returning errors:

```go
if err != nil {
    terminatedAt := time.Now()
    ctx.SendStatus(&api.ActionResult{
        UnitID:            payload.UnitID,
        SpaceID:           payload.SpaceID,
        QueuedOperationID: payload.QueuedOperationID,
        ActionResultBaseMeta: api.ActionResultBaseMeta{
            Action:       api.ActionApply,
            Result:       api.ActionResultApplyFailed,
            Status:       api.ActionStatusFailed,
            Message:      fmt.Sprintf("Failed to write file: %v", err),
            StartedAt:    startTime,
            TerminatedAt: &terminatedAt,
        },
    })
    return fmt.Errorf("failed to write file %s: %w", filepath, err)
}
```

### Progress Updates

For long-running operations, send periodic status updates:

1. Initial "progressing" status when starting
2. Intermediate updates for multi-step operations
3. Final "completed" or "failed" status

## Example Implementations

This filesystem bridge demonstrates basic concepts. Examples of real-world bridges might be:

- **Kubernetes Bridge**: Apply YAML manifests to clusters
- **AWS Bridge**: Manage cloud resources via APIs
- **Database Bridge**: Execute schema migrations
