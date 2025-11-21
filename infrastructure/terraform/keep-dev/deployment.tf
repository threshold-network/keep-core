module "push_deployment_infrastructure" {
  source                 = "git@github.com:thesis/infrastructure.git//terraform/modules/gcp_push_deploy"
  project                = "${module.project.project_id}"
  region                 = "${var.region_data["region"]}"
  vpc_network_name       = "${module.vpc.vpc_network_name}"
  vpc_public_subnet_name = "${module.vpc.vpc_public_subnet_name}"
  vpc_gke_subnet_name    = "${module.gke_cluster.vpc_gke_subnet_name}"

  jumphost {
    name = "${var.jumphost["name"]}"
    zone = "${var.region_data["zone_a"]}"
    tags = "${var.jumphost["tags"]}"
  }

  utility_box {
    name         = "${var.utility_box["name"]}"
    machine_type = "${var.utility_box["machine_type"]}"
    machine_image = "${var.utility_box["machine_image"]}"
    tools        = "${var.utility_box["tools"]}"
    zone         = "${var.region_data["zone_a"]}"
    tags         = "${var.utility_box["tags"]}"
  }

  labels = "${local.labels}"
}

resource "random_id" "ci_publish_to_gcr" {
  byte_length = 2
}

# Service Account CI uses to publish images to GCR
resource "google_service_account" "ci_publish_to_gcr_service_account" {
  project      = "${module.project.project_id}"
  account_id   = "ci-publish-to-gcr-${random_id.ci_publish_to_gcr.hex}"
  display_name = "ci-publish-to-gcr"
}

resource "google_project_iam_member" "ci_publish_to_gcr_service_account" {
  project = "${module.project.project_id}"
  role    = "roles/storage.admin"
  member  = "${local.service_account_prefix}:${google_service_account.ci_publish_to_gcr_service_account.email}"
}