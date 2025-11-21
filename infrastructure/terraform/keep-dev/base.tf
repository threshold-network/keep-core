data "google_client_config" "default" {}

# Configure the Google Cloud provider
provider "google" {
  version = "<= 2.20.3"
  region  = "${var.region_data["region"]}"
}

provider "google-beta" {
  version = "<= 2.20.3"
  region  = "${var.region_data["region"]}"
}

/* Set your locals.
 * Terraform doesn't allow for string interpolation in variable maps.
 * We cheat it by defining a local. A local instance variable mapping
 * allows for string interpolation in maps. Locals are also good for
 * names who are a construct of multiple values, to keep module blocks
 * clean.
*/
locals {
  public_subnet_name     = "${var.environment}-${module.vpc.vpc_subnet_prefix}-pub-${var.region_data["region"]}"
  private_subnet_name    = "${var.environment}-${module.vpc.vpc_subnet_prefix}-pri-${var.region_data["region"]}"
  gke_subnet_name        = "${var.environment}-${module.vpc.vpc_subnet_prefix}-gke-${var.region_data["region"]}"
  service_account_prefix = "serviceAccount"

  labels {
    contact     = "${var.contacts}"
    environment = "${var.environment}"
    vertical    = "${var.vertical}"
  }
}

module "project" {
  source                = "git@github.com:thesis/infrastructure.git//terraform/modules/gcp_project"
  name                  = "${var.project_name}"
  org_id                = "${var.gcp_thesis_org_id}"
  billing_account       = "${var.gcp_thesis_billing_account}"
  project_owner_members = "${var.project_owner_members}"
  labels                = "${local.labels}"
}

# Remote state storage bucket
module "backend_bucket" {
  source   = "git@github.com:thesis/infrastructure.git//terraform/modules/gcp_bucket"
  name     = "${var.backend_bucket_name}"
  project  = "${module.project.project_id}"
  location = "${var.region_data["region"]}"
  labels   = "${local.labels}"
}

# Create vpc and primary subnets
module "vpc" {
  source           = "git@github.com:thesis/infrastructure.git//terraform/modules/gcp_vpc"
  vpc_network_name = "${var.vpc_network_name}"
  project          = "${module.project.project_id}"
  region           = "${var.region_data["region"]}"
  routing_mode     = "${var.routing_mode}"

  public_subnet_name          = "${local.public_subnet_name}"
  public_subnet_ip_cidr_range = "${var.public_subnet_ip_cidr_range}"

  private_subnet_name          = "${local.private_subnet_name}"
  private_subnet_ip_cidr_range = "${var.private_subnet_ip_cidr_range}"
}

resource "google_compute_address" "eth_tx_ropsten_loadbalancer_ip" {
  provider     = "google-beta"
  name         = "${var.eth_tx_ropsten_loadbalancer_name}"
  project      = "${module.project.project_id}"
  region       = "${var.region_data["region"]}"
  address_type = "${upper(var.eth_tx_ropsten_loadbalancer_address_type)}"
  labels       = "${local.labels}"
}

resource "google_compute_address" "eth_miner_ropsten_loadbalancer_ip" {
  provider     = "google-beta"
  name         = "${var.eth_miner_ropsten_loadbalancer_name}"
  project      = "${module.project.project_id}"
  region       = "${var.region_data["region"]}"
  address_type = "${upper(var.eth_miner_ropsten_loadbalancer_address_type)}"
  labels       = "${local.labels}"
}

resource "google_storage_bucket" "keep_dev_contract_data" {
  name          = "keep-dev-contract-data"
  project       = "${module.project.project_id}"
  location      = "US-CENTRAL1"
  storage_class = "REGIONAL"
  labels        = "${local.labels}"

  versioning {
    enabled = true
  }
}

resource "random_id" "ci_get_bucket_object_service_account_random_account_id" {
  byte_length = 2
}

resource "google_service_account" "ci_get_bucket_object_service_account" {
  project      = "${module.project.project_id}"
  account_id   = "ci-get-bucket-object-${random_id.ci_get_bucket_object_service_account_random_account_id.hex}"
  display_name = "ci-get-bucket-object"
}

resource "google_project_iam_member" "ci_get_bucket_object_service_account" {
  project = "${module.project.project_id}"
  role    = "roles/storage.objectViewer"
  member  = "${local.service_account_prefix}:${google_service_account.ci_get_bucket_object_service_account.email}"
}

# resource "kubernetes_namespace" "tbtc" {
#   metadata {
#     name = "tbtc"
#   }
# }

