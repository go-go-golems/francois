variable "zitadel_domain" {
  description = "Canonical ZITADEL issuer hostname without scheme."
  type        = string
  default     = "zitadel.yolo.scapegoat.dev"
}

variable "public_url" {
  description = "Francois portal origin (catalog serve --public-url)."
  type        = string
  default     = "https://francois.yolo.scapegoat.dev"

  validation {
    condition     = startswith(var.public_url, "https://") && !endswith(var.public_url, "/")
    error_message = "public_url must be an HTTPS origin without a trailing slash."
  }
}

variable "allow_register" {
  description = "Allow self-registration in the Francois organization. Disable for an invite-only beta."
  type        = bool
  default     = true
}
