output "contacts" {
  value = "${var.contacts}"
}

output "vertical" {
  value = "${var.vertical}"
}

output "environment" {
  value = "${var.environment}"
}

output "region_data" {
  value = "${var.region_data}"
}

output "project_name" {
  value = "${module.project.project_name}"
}

output "project_id" {
  value = "${module.project.project_id}"
}

output "project_owner_members" {
  value = "${var.project_owner_members}"
}

output "backend_bucket_name" {
  value = "${module.backend_bucket.bucket_name}"
}

output "vpc_network_name" {
  value = "${module.vpc.vpc_network_name}"
}

output "vpc_network_gateway_ip" {
  value = "${module.vpc.vpc_network_gateway_ip}"
}

output "vpc_public_subnet_name" {
  value = "${module.vpc.vpc_public_subnet_name}"
}

output "vpc_private_subnet_name" {
  value = "${module.vpc.vpc_private_subnet_name}"
}

output "eth_tx_ropsten_loadbalancer_external_ip" {
  value = "${google_compute_address.eth_tx_ropsten_loadbalancer_ip.address}"
}
