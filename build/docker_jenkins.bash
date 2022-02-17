#!/usr/bin/env bash
# This script was copied from 'terraform-provider-akamai'
#
# This script will build the provider and associated library after checking out from git on jenkins.
#
# It uses the same docker image for all builds unless RELOAD_DOCKER_IMAGE parameter is set true.

# Script will end immediately when some command exits with a non-zero exit code.
set -e

CLI_TERRAFORM_BRANCH_NAME="${1:-develop}"
PROVIDER_BRANCH_NAME="${2:-develop}"
EDGEGRID_BRANCH_NAME_V2="${3:-v2}"
CLI_BRANCH_NAME="${4:-develop}"
EDGEGRID_BRANCH_NAME_V1="${5:-develop}"
RELOAD_DOCKER_IMAGE="${6:-false}"


TIMEOUT="20m"
# Recalculate DOCKER_IMAGE_SIZE if any changes to dockerfile.
DOCKER_IMAGE_SIZE="642345946"

SSH_PRV_KEY="$(cat ~/.ssh/id_rsa)"
SSH_PUB_KEY="$(cat ~/.ssh/id_rsa.pub)"
SSH_KNOWN_HOSTS="$(cat ~/.ssh/known_hosts)"

WORKDIR="${WORKDIR-$(pwd)}"
echo "WORKDIR is $WORKDIR"
TERRAFORM_VERSION="1.0.4"

STASH_SERVER=git.source.akamai.com
GIT_IP=$(dig +short $STASH_SERVER)
[ -z "$GIT_IP" ] && echo "Aborting - Can not reach $STASH_SERVER." && exit 1 || echo "Resolved $STASH_SERVER, preparing build"

eTAG="$(git describe --tags --always)"
PROVIDER_BRANCH_HASH="$(git rev-parse --short HEAD)"
echo "Making build on branch $PROVIDER_BRANCH_NAME at hash $PROVIDER_BRANCH_HASH with tag $eTAG"

cp "$HOME"/.edgerc "$WORKDIR"/.edgerc
sed -i -e "1s/^.*$/[default]/" "$WORKDIR"/.edgerc

docker rm -f akatf-container 2> /dev/null || true

# Remove docker image if RELOAD_DOCKER_IMAGE is true
if [[ "$RELOAD_DOCKER_IMAGE" == true ]]; then
  echo "Removing docker image terraform/akamai:terraform-provider-akamai if exists"
  docker image rm -f terraform/akamai:terraform-provider-akamai 2> /dev/null || true
fi

if [[ "$(docker images -q terraform/akamai:terraform-provider-akamai 2> /dev/null)" == "" ||
      "$(docker inspect -f '{{ .Size }}' terraform/akamai:terraform-provider-akamai)" != "$DOCKER_IMAGE_SIZE" ]]; then
  echo "Building new image terraform/akamai:terraform-provider-akamai"
  DOCKER_BUILDKIT=1 docker build \
    -f build/Dockerfile \
    --build-arg TERRAFORM_VERSION=${TERRAFORM_VERSION} \
    --no-cache \
    -t terraform/akamai:terraform-provider-akamai .
fi

echo "Creating docker container"
docker run -d -it --name akatf-container --entrypoint "/usr/bin/tail" \
        -e TF_LOG=DEBUG \
        -e TF_LOG_PATH="provider.log" \
        -e COVERMODE="atomic" \
        -e CLI_TERRAFORM_BRANCH_NAME="$CLI_TERRAFORM_BRANCH_NAME" \
        -e EDGEGRID_BRANCH_NAME_V1="$EDGEGRID_BRANCH_NAME_V1" \
        -e EDGEGRID_BRANCH_NAME_V2="$EDGEGRID_BRANCH_NAME_V2" \
        -e PROVIDER_BRANCH_NAME="$PROVIDER_BRANCH_NAME" \
        -e CLI_BRANCH_NAME="$CLI_BRANCH_NAME" \
        -e SSH_PUB_KEY="${SSH_PUB_KEY}" \
        -e SSH_PRV_KEY="${SSH_PRV_KEY}" \
        -e SSH_KNOWN_HOSTS="${SSH_KNOWN_HOSTS}" \
        -e TIMEOUT="$TIMEOUT" \
        -v "$HOME"/.ssh/id_rsa=/root/id_rsa \
        -v "$HOME"/.ssh/id_rsa.pub=/root/id_rsa.pub \
        -v "$HOME"/.ssh/known_hosts=/root/known_hosts \
        -v "$WORKDIR"/.edgerc:/root/.edgerc:ro \
        -w /tf/ \
        terraform/akamai:terraform-provider-akamai -f /dev/null

docker exec akatf-container sh -c 'echo "$SSH_KNOWN_HOSTS" > /root/.ssh/known_hosts;
                                   echo "$SSH_PUB_KEY" > /root/.ssh/id_rsa.pub;
                                   echo "$SSH_PRV_KEY" > /root/.ssh/id_rsa;
                                   chmod 700 /root/.ssh;
                                   chmod 600 /root/.ssh/id_rsa;
                                   chmod 644 /root/.ssh/id_rsa.pub /root/.ssh/known_hosts'
echo "Cloning repos"
docker exec akatf-container sh -c 'git clone ssh://git@git.source.akamai.com:7999/devexp/terraform-provider-akamai.git;
                                   git clone ssh://git@git.source.akamai.com:7999/devexp/akamaiopen-edgegrid-golang.git edgegrid-v1;
                                   git clone ssh://git@git.source.akamai.com:7999/devexp/akamaiopen-edgegrid-golang.git edgegrid-v2;
                                   git clone ssh://git@git.source.akamai.com:7999/devexp/cli-terraform.git;
                                   git clone ssh://git@git.source.akamai.com:7999/devexp/cli.git'

echo "Checkout branches"
docker exec akatf-container sh -c 'cd edgegrid-v1; git checkout ${EDGEGRID_BRANCH_NAME_V1};
                                   cd ../edgegrid-v2; git checkout ${EDGEGRID_BRANCH_NAME_V2};
                                   cd ../terraform-provider-akamai; git checkout ${PROVIDER_BRANCH_NAME};
                                   go mod edit -replace github.com/akamai/AkamaiOPEN-edgegrid-golang/v2=../edgegrid-v2;
                                   go mod tidy;
                                   cd ../cli-terraform; git checkout ${CLI_TERRAFORM_BRANCH_NAME};
                                   go mod edit -replace github.com/akamai/AkamaiOPEN-edgegrid-golang=../edgegrid-v1;
                                   go mod edit -replace github.com/akamai/AkamaiOPEN-edgegrid-golang/v2=../edgegrid-v2;
                                   go mod tidy;
                                   cd ../cli; git checkout ${CLI_BRANCH_NAME};
                                   go mod edit -replace github.com/akamai/cli=../cli;
                                   go mod tidy'



echo "Creating docker build for terraform-provider-akamai"
docker exec akatf-container sh -c 'cd terraform-provider-akamai; go install -tags all;
                                   mkdir -p /root/.terraform.d/plugins/registry.terraform.io/akamai/akamai/${PROVIDER_VERSION}/linux_amd64;
                                   cp /go/bin/terraform-provider-akamai /root/.terraform.d/plugins/registry.terraform.io/akamai/akamai/${PROVIDER_VERSION}/linux_amd64/terraform-provider-akamai_v${PROVIDER_VERSION}'

echo "Running checks"
docker exec akatf-container sh -c 'cd cli-terraform; make all'

docker rm -f akatf-container 2> /dev/null || true
