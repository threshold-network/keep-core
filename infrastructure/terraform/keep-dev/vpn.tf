resource "helm_release" "openvpn" {
  name      = "${var.openvpn["name"]}"
  namespace = "${var.openvpn["namespace"]}"
  chart     = "${var.openvpn["helm_chart"]}"
  version   = "${var.openvpn["helm_chart_version"]}"
  keyring   = ""

  set {
    name  = "openvpn.redirectGateway"
    value = "${var.openvpn["route_all_traffic_through_vpn"]}"
  }

  # Netmask is not configurable because GKE requires /28 for master subnet range.
  set {
    name  = "openvpn.serverConf"
    value = "push \"route ${var.openvpn["gke_master_cidr"]} 255.255.255.240\""
  }
}
