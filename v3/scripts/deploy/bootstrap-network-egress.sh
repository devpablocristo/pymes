#!/usr/bin/env bash
set -euo pipefail

# Plans a shared Direct VPC egress subnet and one shared public Cloud NAT for
# Pymes v3 STG/PRD. The default is deliberately read-only because Cloud NAT has
# a recurring cost even when application traffic is low.

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
# shellcheck source=gcp-target-policy.sh
source "$script_dir/gcp-target-policy.sh"

project=${PYMES_GCP_PROJECT:-pymes-dev-352318}
region=${PYMES_GCP_REGION:-us-central1}
network=${PYMES_VPC_NETWORK:-default}
subnet=${PYMES_VPC_SUBNET:-pymes-v3-serverless}
subnet_cidr=${PYMES_VPC_SUBNET_CIDR:-10.120.0.0/24}
router=${PYMES_VPC_NAT_ROUTER:-pymes-v3-serverless}
nat=${PYMES_VPC_NAT_NAME:-pymes-v3-serverless}
apply=${PYMES_NETWORK_BOOTSTRAP_APPLY:-false}

pymes_require_canonical_project_region "$project" "$region"

case "$apply" in
  true|false) ;;
  *) echo "PYMES_NETWORK_BOOTSTRAP_APPLY must be true or false" >&2; exit 2 ;;
esac
for resource_name in "$network" "$subnet" "$router" "$nat"; do
  if [[ ! "$resource_name" =~ ^[a-z]([-a-z0-9]*[a-z0-9])?$ ]]; then
    echo "network resource names must be valid explicit GCP names" >&2
    exit 2
  fi
done
if [[ ! "$subnet_cidr" =~ ^([0-9]{1,3}\.){3}[0-9]{1,3}/([0-9]{1,2})$ ]]; then
  echo "PYMES_VPC_SUBNET_CIDR must be an IPv4 CIDR" >&2
  exit 2
fi
prefix_length=${BASH_REMATCH[2]}
if (( prefix_length < 20 || prefix_length > 26 )); then
  echo "PYMES_VPC_SUBNET_CIDR prefix must be between /20 and /26" >&2
  exit 2
fi
address=${subnet_cidr%/*}
IFS='.' read -r octet1 octet2 octet3 octet4 <<<"$address"
for octet in "$octet1" "$octet2" "$octet3" "$octet4"; do
  if [[ ! "$octet" =~ ^[0-9]+$ ]] || (( 10#$octet > 255 )); then
    echo "PYMES_VPC_SUBNET_CIDR contains an invalid IPv4 address" >&2
    exit 2
  fi
done
if ! (( 10#$octet1 == 10 ||
        (10#$octet1 == 172 && 10#$octet2 >= 16 && 10#$octet2 <= 31) ||
        (10#$octet1 == 192 && 10#$octet2 == 168) )); then
  echo "PYMES_VPC_SUBNET_CIDR must use an RFC1918 private IPv4 range" >&2
  exit 2
fi
address_value=$(( (10#$octet1 << 24) |
                  (10#$octet2 << 16) |
                  (10#$octet3 << 8) |
                  10#$octet4 ))
host_bits=$((32 - prefix_length))
host_mask=$(((1 << host_bits) - 1))
if (( (address_value & host_mask) != 0 )); then
  echo "PYMES_VPC_SUBNET_CIDR must be aligned to its prefix" >&2
  exit 2
fi
pymes_require_canonical_network_target \
  "$network" "$subnet" "$subnet_cidr" "$router" "$nat"

echo "NETWORK PLAN project=$project region=$region network=$network"
echo "NETWORK PLAN subnet=$subnet cidr=$subnet_cidr private_google_access=true"
echo "NETWORK PLAN router=$router nat=$nat allocation=automatic scope=subnet-only ranges=all"
echo "COST recurring Cloud NAT gateway, public IPv4 and processed traffic charges apply"

if [[ "$apply" == "false" ]]; then
  echo "PLAN ONLY resources_created=0; set PYMES_NETWORK_BOOTSTRAP_APPLY=true and acknowledge the recurring cost to apply"
  exit 0
fi
if [[ "${PYMES_NETWORK_COST_ACK:-}" != "I_ACCEPT_RECURRING_CLOUD_NAT_COST" ]]; then
  echo "set PYMES_NETWORK_COST_ACK=I_ACCEPT_RECURRING_CLOUD_NAT_COST before creating billed network resources" >&2
  exit 2
fi

export CLOUDSDK_CORE_PROJECT="$project"
gcloud services enable compute.googleapis.com run.googleapis.com \
  --project="$project" >/dev/null
gcloud compute networks describe "$network" \
  --project="$project" >/dev/null

if gcloud compute networks subnets describe "$subnet" \
  --project="$project" --region="$region" >/dev/null 2>&1; then
  subnet_json=$(gcloud compute networks subnets describe "$subnet" \
    --project="$project" --region="$region" --format=json)
  jq -e \
    --arg network "$network" \
    --arg region "$region" \
    --arg cidr "$subnet_cidr" \
    '
      (.network | endswith("/global/networks/" + $network)) and
      (.region | endswith("/regions/" + $region)) and
      .ipCidrRange == $cidr
    ' <<<"$subnet_json" >/dev/null || {
    echo "existing subnet $subnet does not match network, region or CIDR; refusing to replace it" >&2
    exit 1
  }
  if ! jq -e '.privateIpGoogleAccess == true' \
    <<<"$subnet_json" >/dev/null; then
    gcloud compute networks subnets update "$subnet" \
      --project="$project" --region="$region" \
      --enable-private-ip-google-access >/dev/null
  fi
else
  gcloud compute networks subnets create "$subnet" \
    --project="$project" --region="$region" \
    --network="$network" --range="$subnet_cidr" \
    --stack-type=IPV4_ONLY \
    --enable-private-ip-google-access >/dev/null
fi

if gcloud compute routers describe "$router" \
  --project="$project" --region="$region" >/dev/null 2>&1; then
  router_json=$(gcloud compute routers describe "$router" \
    --project="$project" --region="$region" --format=json)
  jq -e --arg network "$network" \
    '.network | endswith("/global/networks/" + $network)' \
    <<<"$router_json" >/dev/null || {
    echo "existing router $router belongs to a different VPC; refusing to replace it" >&2
    exit 1
  }
else
  gcloud compute routers create "$router" \
    --project="$project" --region="$region" \
    --network="$network" >/dev/null
fi

if ! gcloud compute routers nats describe "$nat" \
  --router="$router" --project="$project" --region="$region" \
  >/dev/null 2>&1; then
  gcloud compute routers nats create "$nat" \
    --router="$router" --project="$project" --region="$region" \
    --nat-custom-subnet-ip-ranges="${subnet}:ALL" \
    --auto-allocate-nat-external-ips \
    --enable-dynamic-port-allocation \
    --min-ports-per-vm=64 \
    --max-ports-per-vm=4096 >/dev/null
else
  nat_json=$(gcloud compute routers nats describe "$nat" \
    --router="$router" --project="$project" --region="$region" \
    --format=json)
  if ! jq -e \
    --arg subnet "$subnet" \
    --arg region "$region" \
    '
      .sourceSubnetworkIpRangesToNat == "LIST_OF_SUBNETWORKS" and
      (.subnetworks | length) == 1 and
      (.subnetworks[0].name |
        endswith("/regions/" + $region + "/subnetworks/" + $subnet)) and
      (
        .natIpAllocateOption == "AUTO_ONLY" or
        (
          .natIpAllocateOption == "MANUAL_ONLY" and
          ((.natIps // []) | length) > 0
        )
      ) and
      (
        (.endpointTypes // ["ENDPOINT_TYPE_VM"]) |
        index("ENDPOINT_TYPE_VM") != null
      )
    ' <<<"$nat_json" >/dev/null; then
    echo "existing Cloud NAT $router/$nat is not exclusively owned by $subnet; refusing to rewrite it" >&2
    exit 1
  fi
  if ! jq -e \
    '.subnetworks[0].sourceIpRangesToNat | index("ALL_IP_RANGES") != null' \
    <<<"$nat_json" >/dev/null; then
    gcloud compute routers nats update "$nat" \
      --router="$router" --project="$project" --region="$region" \
      --nat-custom-subnet-ip-ranges="${subnet}:ALL" \
      --enable-dynamic-port-allocation \
      --min-ports-per-vm=64 \
      --max-ports-per-vm=4096 >/dev/null
  fi
fi

nat_json=$(gcloud compute routers nats describe "$nat" \
  --router="$router" --project="$project" --region="$region" \
  --format=json)
jq -e \
  --arg subnet "$subnet" \
  --arg region "$region" \
  '
    .sourceSubnetworkIpRangesToNat == "LIST_OF_SUBNETWORKS" and
    any(
      .subnetworks[]?;
      (.name | endswith("/regions/" + $region + "/subnetworks/" + $subnet)) and
      (.sourceIpRangesToNat | index("ALL_IP_RANGES") != null)
    ) and
    (
      .natIpAllocateOption == "AUTO_ONLY" or
      (
        .natIpAllocateOption == "MANUAL_ONLY" and
        ((.natIps // []) | length) > 0
      )
    ) and
    (
      (.endpointTypes // ["ENDPOINT_TYPE_VM"]) |
      index("ENDPOINT_TYPE_VM") != null
    )
  ' <<<"$nat_json" >/dev/null || {
  echo "existing Cloud NAT $router/$nat does not provide public NAT for all ranges of $subnet" >&2
  exit 1
}

subnet_json=$(gcloud compute networks subnets describe "$subnet" \
  --project="$project" --region="$region" --format=json)
jq -e '.privateIpGoogleAccess == true' <<<"$subnet_json" >/dev/null

echo "NETWORK READY network=$network subnet=$subnet cidr=$subnet_cidr private_google_access=true public_nat=$router/$nat"
printf 'PYMES_VPC_NETWORK=%s\n' "$network"
printf 'PYMES_VPC_SUBNET=%s\n' "$subnet"
printf 'PYMES_VPC_SUBNET_CIDR=%s\n' "$subnet_cidr"
printf 'PYMES_VPC_NAT_ROUTER=%s\n' "$router"
printf 'PYMES_VPC_NAT_NAME=%s\n' "$nat"
