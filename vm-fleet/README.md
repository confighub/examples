# Managing a VM fleet with ConfigHub

This example shows how to manage a fleet of single-instance EC2 instances using ConfigHub

## Scenario

While containers are all the rage, almost every organization is also managing various fleets of VMs. It is common to build some kind of tooling layer on top of the raw VM management APIs to increase productivity and reduce errors.

Unfortunately these tooling layers often turn into "straight jackets" in that they don't allow for any flexibility that was not included in the initial design of the tool. As a result, when one team wants something slightly different from what the tool offers, it becomes a change request for the tool itself which can become high-risk because the tool is so widely deployed. Hence it requires extra scrutiny which requires extra effort which pushes the request down the priority list.

## Setup

For this example, you will need an AWS account and be willing to spend a few cents on temporarily spinning up EC2 instances. The scripts assume that you have the AWS CLI installed and it will use your default profile to execute commands. You can set `AWS_PROFILE` prior to running the scripts to control which of your accounts is being used.

The `setup-iam` script will use your AWS account to create a new IAM user and grant the permissions necessecary for this example to work. Please consult [aws/policy.json](aws/policy.json) to review what permissions are being granted.

The credentials for the IAM user will be stored locally in var/aws-credentials.ini and added to a locally running Kind cluster as a secret. They will not be transmitted anywhere outside of your local machine.

If you don't want to follow the step-by-step sections below, you can set up everything with a single script:

    bin/setup

Whether you use this all-in-one script or the detailed steps below, it is highly recommended that you read the source of the scripts to understand what is going on. They are all very simple scripts that should be easy to read.

### Set up ConfigHub

First run:

    bin/setup-units

This will create a new space with a unique name (and save its name in `.cub-space` for later). Then it will create the following Config Units:

* Crossplane system (via a Helm install)
* Crossplane providers: S3, IAM, EC2 and AutoScaling
* Crossplane config: AWS credentials
* VM fleet baseline infrastructure: Shared AWS resources and an instance base unit


### Set up a local Kind cluster

We will be using Crossplane to manage the VM fleet. So while the fleet runs in AWS, we still need a place to run the Crossplane control plane and the easiest option is a local Kind cluster. Assuming you have Kind and its dependencies installed, run this script to set it up:

    bin/setup-cluster

This will boot a Kind cluster, install a ConfigHub worker with proper credentials and associate the config units created in the previous step with the target of this cluster.

### Apply Crossplane and network units

Now the first set of Units to the cluster:

    cub unit apply --space $space --where "Labels.type = 'crossplane'"

This will apply the crossplane system, providers, provider config and default network "tracking" resources. You may need to rerun the apply a few times before all units are applied. If you check the `bin/install-units` script, you'll see how we set labels when creating units. We can use this for many different purposes such as selecting which units to apply.

### Set networking information

The network unit contains two resources: default-vpc and default-subnet-a. These are a special kind of Crossplane resource that don't create any new resources in AWS. Instead they act as data sources that provides the default VPC ID and default subnet ID in availability zone us-west-2a for your AWS account. Run the following script to retrieve these values and set them in `shared-resources` and `instance-base`:

    bin/set-network-info

ConfigHub doesn't use variables. Instead information such as network identifiers are "propagated" between units using the [Needs/Provides](https://docs.confighub.com/concepts/needsprovides/) mechanism and written literally into the config resource data. This mechanism currently does not support propagation in this particular case so we use a script instead. But in the future, the links between units will be used to propagate the values automatically.

### Apply the shared resources unit

Now the shared resources unit can be applied

    cub unit apply shared-resources --wait=false

We do not need to apply the `instance-base` unit because it is a base unit that we will clone other units from when we create instances.

### Create a filter

Create a convenient filter that selects just the units we care about:

    cub filter create instances Unit --where-field "Labels.type='instance'"

This filter will be used many of the commands later on.

## Scenario Tasks

With all the steps above completed, you are ready to do some VM management

### Run a VM

Create a new VM config with:

    bin/create server1

This will create a clone of the instance-base unit and set various fields in the clone to uniquely identify it as server1.

Now start the VM with:

    bin/start server1

This will update the cloned unit changing desired capacity from 0 to 1 in the AWS autoscaling group and then applying it. Now AWS will boot the instance. You can check on instances with this command:

    bin/list

This command uses your AWS CLI directly so it's not a ConfigHub thing per se. It corresponds to a typical operational tool. Once the instance is available, you can run:

    bin/hello server1

This will connect to the instance and print some information including the instance name. You can use the name or other tags to customize each instance to perform different roles.

### Run a second VM

You can repeat the steps above to add more VMs to your fleet, e.g:

    bin/create server2
    bin/start server2

### Update config for all instances

You can list all instances with:

    cub unit list --filter instances

You can see the config for each unit with:

    cub unit get <unit name> --data-only

What if you want to change the config for all units? Isn't that complicated when ConfigHub maintains separate copies of the config? This is where clones come into the picture. If you want to propagate a change to all clones, you simply make the change in the base and then "upgrade" the clones. For example, maybe you want all instances to be t4g.small instead of t4g.micro:

    bin/set-size base tg4.small

This script sets the instance-type field in the specified config unit. Now we can check if any of our cloned units needs to be upgraded.

    cub unit list --filter instances --columns Name,UpgradeNeeded

All units need an upgrade now because we made a change to the base. We can upgrade them one by one:

    cub unit update server-instance1 --upgrade
    ...

or in bulk:

    cub unit update --patch --upgrade --filter instances

Note that in bulk mode, the update command needs the `patch` flag. You can now apply all changes with

    cub unit apply --filter instances

### Update config for a single instance

One of the most valuable capabilities of ConfigHub is that you can update the config of a single unit without affecting other variants. Let's say that you only need to increase the size of one of your instances. You simply edit the config of only that instance:

    bin/set-size server1 t4g.medium
    cub unit apply instance-server1

Note that instance size changes will not take effect in AWS until a new EC2 instance is created, so if you want to see it in action, run `bin/stop`, wait a few seconds and then run `bin/start` again.

### Prevent clobbering

A common problem when making changes to one resource that is part of a variant set is how to prevent the changes from being overwritten later on. ConfigHub handles this for you by tracking changes across variants. Let's say that bumping all instance sizes to tg4.small was a mistake and someone decides to set it back to t4g.micro. Yet, your particular instance really needs to be a t4g.medium as you specified previously. Here is how this is handled in ConfigHub: Change the size in the base:

    bin/set-size base t4g.micro

Upgrade all clones:

    cub unit update --patch --upgrade --filter instances

Check to see what happened to server1:

    cub unit diff instance-server1 --from=-1

The instance type did not change because it has been edited in the local clone and therefore the local clone now "owns" this field.

### Make an unanticipated change

As mentioned in the introduction, ConfigHub prevents tooling layers from becoming "straight jackets" where you can only control the things that were anticipated by the tooling layer.

In this example, the bin directory represents the tooling layer. It has commands for things like starting, stopping and changing the size of an instance. But what if you want to change something else?

For example, what if you are having trouble with a particular instance and you find out you need to increase the healthcheck grace period. With ConfigHub this is easy, simply edit the raw config for your clone:

    cub unit edit instance-server1

Alternatively, you can use `cub run set-int-path` to programmatically insert or update the `healthCheckGracePeriod` field in the `forProvider` section:

    cub run set-int-path \
            --unit instance-server1 \
            --resource-type autoscaling.aws.upbound.io/v1beta2/AutoscalingGroup \
            --path spec.forProvider.healthCheckGracePeriod \
            --attribute-value 600

This will set the health check grace period to 600 seconds for the AutoScalingGroup resource in the specified unit. The `set-int-path` command is one of many commands that perform in-place edits on the YAML configuration, making it easy to update specific fields without manually editing the entire config.

The raw config can contain any configuration field supported by the underlying infrastructure API. In this case the API is the [Crossplane AutoScalingGroup](https://marketplace.upbound.io/providers/crossplane-contrib/provider-aws/v0.54.2/resources/autoscaling.aws.crossplane.io/AutoScalingGroup/v1beta1). You can set any config field specified in this resource.

After you change the grace period to, say 600, you can still safely receive upgrades from the clone base thanks to the field ownership mechanism that you saw in action in the previous task.

