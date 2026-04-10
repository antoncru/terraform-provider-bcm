# Variables for BCM CMDevice Power Action Example

variable "device_uuid" {
  type        = string
  description = "Target device UUID for power operations (powerOperation requires UUIDs)"
  default     = ""
}

variable "bcm_password" {
  type        = string
  description = "BCM password (use TF_VAR_bcm_password environment variable)"
  sensitive   = true
  default     = ""
}
