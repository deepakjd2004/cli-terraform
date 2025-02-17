terraform {
  required_providers {
    akamai = {
      source  = "akamai/akamai"
      version = ">= 3.1.0"
    }
  }
  required_version = ">= 0.13"
}

provider "akamai" {
  edgerc         = var.edgerc_path
  config_section = var.config_section
}

resource "akamai_cps_dv_enrollment" "enrollment_id_1" {
  common_name                           = "test.akamai.com"
  allow_duplicate_common_name           = false
  secure_network                        = "enhanced-tls"
  sni_only                              = true
  acknowledge_pre_verification_warnings = false
  admin_contact {
    first_name       = "R1"
    last_name        = "D1"
    organization     = "Akamai"
    email            = "r1d1@akamai.com"
    phone            = "123123123"
    address_line_one = "150 Broadway"
    city             = "Cambridge"
    region           = "MA"
    postal_code      = "12345"
    country_code     = "US"
  }
  csr {
    country_code        = "US"
    city                = "Cambridge"
    organization        = "Akamai"
    organizational_unit = "WebEx"
    state               = "MA"
  }
  network_configuration {
    geography = "core"
  }
  signature_algorithm = "SHA-256"
  tech_contact {
    first_name       = "R2"
    last_name        = "D2"
    organization     = "Akamai"
    email            = "r2d2@akamai.com"
    phone            = "123123123"
    address_line_one = "150 Broadway"
    city             = "Cambridge"
    region           = "MA"
    postal_code      = "12345"
    country_code     = "US"
  }
  organization {
    name             = "Akamai"
    phone            = "321321321"
    address_line_one = "150 Broadway"
    city             = "Cambridge"
    region           = "MA"
    postal_code      = "12345"
    country_code     = "US"
  }
  contract_id = "ctr_1"
}