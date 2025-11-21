resource "google_compute_address" "cloud_nat" {
  name         = "keep-dev-cloud-nat-external-ip"
  project      = "${module.project.project_id}"
  region       = "${var.region_data["region"]}"
  address_type = "EXTERNAL"
  network_tier = "PREMIUM"
}

resource "google_compute_router" "cloud_nat" {
  name    = "keep-dev-cloud-nat-router"
  project = "${module.project.project_id}"
  region  = "${var.region_data["region"]}"
  network = "${module.vpc.vpc_network_name}"
}

resource "google_compute_router_nat" "cloud_nat" {
  name                               = "keep-dev-cloud-nat"
  project                            = "${module.project.project_id}"
  region                             = "${var.region_data["region"]}"
  router                             = "${google_compute_router.cloud_nat.name}"
  nat_ip_allocate_option             = "MANUAL_ONLY"
  nat_ips                            = ["${google_compute_address.cloud_nat.self_link}"]
  source_subnetwork_ip_ranges_to_nat = "LIST_OF_SUBNETWORKS"

  subnetwork {
    name                    = "${module.vpc.vpc_private_subnet_self_link}"
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }

  subnetwork {
    name                    = "${module.gke_cluster.vpc_gke_subnet_self_link}"
    source_ip_ranges_to_nat = ["ALL_IP_RANGES"]
  }
}
