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

output "nat_gateway_external_ips" {
  value = "${module.nat_gateway_external_ips.ip_address_set}"
}

output "nat_gateway_zone_a_instance" {
  value = "${module.nat_gateway_zone_a.instance}"
}

output "nat_gateway_zone_b_instance" {
  value = "${module.nat_gateway_zone_b.instance}"
}

output "nat_gateway_zone_c_instance" {
  value = "${module.nat_gateway_zone_c.instance}"
}

output "nat_gateway_region_route_tag" {
  value = "${module.nat_gateway_zone_a.routing_tag_regional}"
}

output "nat_gateway_zone_a_route_tag" {
  value = "${module.nat_gateway_zone_a.routing_tag_zonal}"
}

output "nat_gateway_zone_b_route_tag" {
  value = "${module.nat_gateway_zone_b.routing_tag_zonal}"
}

output "nat_gateway_zone_c_route_tag" {
  value = "${module.nat_gateway_zone_c.routing_tag_zonal}"
}
