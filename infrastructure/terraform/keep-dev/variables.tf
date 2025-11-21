# env vars
variable "gcp_thesis_org_id" {
  description = "The ID for the organization the project will be created under. Local ENV VAR"
}

variable "gcp_thesis_billing_account" {
  description = "The billing account to associate with your project.  Must be associated with org already. Local ENV VAR"
}

# generic vars
variable "region_data" {
  description = "Region and zone info."

  default {
    region = "us-central1"
    zone_a = "us-central1-a"
    zone_b = "us-central1-b"
    zone_c = "us-central1-c"
    zone_f = "us-central1-f"
  }
}

variable "contacts" {
  description = "The person(s) who contribute to this tf stack."
  default     = "sthompson22"
}

variable "vertical" {
  description = "Name of the vertical that the generated resources belong to.  e.g. cfc, keep"
  default     = "keep"
}

variable "environment" {
  description = "Environment you're creating resources in.  Usually project name"
  default     = "keep-dev"
}

# project vars
variable "project_name" {
  description = "Name for the project."
  default     = "keep-dev"
}

variable "project_owner_members" {
  description = "List of service and user accounts to add with owner permissions to project."

  default = [
    "user:sloan.thompson@thesis.co",
    "user:antonio.salazarcardozo@thesis.co",
  ]
}

# module IAM members: editor
variable "editor_iam_role" {
  default = "roles/editor"
}

variable "editor_iam_members" {
  default = ["user:jakub.nowakowski@thesis.co", "user:nicholas.evans@thesis.co", "user:nik.grinkevich@thesis.co", "user:piotr.dyraga@thesis.co", "user:rafal.czajkowski@thesis.co", "user:dymitr.paremski@thesis.co"]
}

# module IAM members: storage.objectViewer
variable "storage_objectviewer_iam_role" {
  default = "roles/storage.objectViewer"
}

variable "storage_objectviewer_iam_members" {
  default = []
}

# bucket vars
## backend bucket
variable "backend_bucket_name" {
  description = "Bucket for storing keep-dev Terraform remote state."
  default     = "keep-dev-tf-backend-bucket"
}

# network vars
## vpc vars
### vpc-network
variable "vpc_network_name" {
  description = "The name for your vpc-network"
  default     = "keep-dev-vpc-network"
}

variable "routing_mode" {
  description = "The dynamic router mode for the vpc-network."
  default     = "regional"
}

### vpc-subnet
#### public subnet
variable "public_subnet_ip_cidr_range" {
  description = "IP address range assigned to the public subnet."
  default     = "10.0.0.0/16"
}

#### private subnet
variable "private_subnet_ip_cidr_range" {
  description = "IP address range assigned to the private subnet."
  default     = "10.1.0.0/16"
}

## nat gateway vars
### external IP address vars
variable "nat_gateway_ip_allocation_count" {
  description = "Generate 3 external IPs, one for each NAT instance."
  default     = "3"
}

variable "nat_gateway_ip_name" {
  description = "The name for your nat gateway IPs."
  default     = "keep-dev-nat-gateway-external-ip"
}

variable "nat_gateway_ip_address_type" {
  description = "external or internal, for NATs we use external."
  default     = "external"
}

# gke
variable "gke_cluster" {
  description = "The Google managed part of the cluster configuration."

  default {
    name                                = "keep-dev"
    private_cluster                     = true
    master_ipv4_cidr_block              = "172.16.0.0/28"
    master_private_endpoint             = "172.16.0.2"
    daily_maintenance_window_start_time = "00:00"
    network_policy_enabled              = false
    network_policy_provider             = "PROVIDER_UNSPECIFIED"
    logging_service                     = "logging.googleapis.com/kubernetes"
    monitoring_service                  = "monitoring.googleapis.com/kubernetes"
  }
}

variable "gke_node_pool" {
  description = "A node pool for the gke cluster."

  default {
    name         = "default-node-pool"
    node_count   = "1"
    machine_type = "n1-standard-2"
    disk_type    = "pd-ssd"
    disk_size_gb = 100
    auto_repair  = "true"
    auto_upgrade = "true"
    oauth_scopes = "https://www.googleapis.com/auth/compute,https://www.googleapis.com/auth/devstorage.read_only,https://www.googleapis.com/auth/logging.write,https://www.googleapis.com/auth/monitoring"
  }
}

variable "gke_subnet" {
  description = "Subnet for deploying GKE cluster resources."

  default {
    primary_ip_cidr_range = "10.2.0.0/16"

    services_secondary_range_name    = "keep-dev-gke-services-secondary-range"
    services_secondary_ip_cidr_range = "10.102.100.0/24"

    cluster_secondary_range_name    = "keep-dev-gke-cluster-secondary-range"
    cluster_secondary_ip_cidr_range = "10.102.0.0/20"
  }
}

variable "eth_tx_ropsten_loadbalancer_name" {
  description = "The name for your ropsten tx node IP."
  default     = "keep-dev-eth-tx-ropsten-loadbalancer-external-ip"
}

variable "eth_tx_ropsten_loadbalancer_address_type" {
  description = "Internet facing or not. internal or external"
  default     = "external"
}

variable "eth_miner_ropsten_loadbalancer_name" {
  description = "The name for your ropsten miner IP."
  default     = "keep-dev-eth-miner-ropsten-loadbalancer-external-ip"
}

variable "eth_miner_ropsten_loadbalancer_address_type" {
  description = "Internet facing or not. internal or external"
  default     = "external"
}

# helm_release openvpn
variable "openvpn" {
  description = "Configuration values for the keep-prd VPN server."

  default {
    name                          = "openvpn"
    namespace                     = "default"
    helm_chart                    = "stable/openvpn"
    helm_chart_version            = "4.2.2"
    route_all_traffic_through_vpn = "false"
    gke_master_cidr               = "172.16.0.0"
  }
}

# deployment infrastructure
## push

# gcp_deploy
variable "jumphost" {
  default {
    name = "keep-dev-jumphost"
    tags = "public-subnet"
  }
}

variable "utility_box" {
  default {
    name          = "keep-dev-utility-box"
    tags          = "gke-subnet"
    machine_type  = "g1-small"
    machine_image = "ubuntu-os-cloud/ubuntu-1804-lts"
    tools         = "kubectl, helm, jq, nodejs, geth"
  }
}
