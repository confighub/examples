# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: stable JSON plan for the example
- proves:
  - which spaces will be created
  - which components participate
  - which layered dimensions are used
  - whether a target was provided

## ConfigHub State Contracts

### `cub unit get --space <prefix>-recipe-eks-h100-ubuntu-training --json recipe-eks-h100-ubuntu-training-stack`

- mutates: no
- output shape: JSON object for the recipe manifest unit
- proves: the stack-level recipe receipt exists in ConfigHub

### `cub unit get --space <prefix>-deploy-cluster-a --json gpu-operator-cluster-a`

- mutates: no
- output shape: JSON object for one deployment unit
- proves:
  - the final deployment variant exists
  - target binding is inspectable if present

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-gpu-eks-h100-training'"`

- mutates: no
- output shape: text tree
- proves: the layered GPU ancestry exists
