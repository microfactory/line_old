variable "project" {}
variable "version" {}
variable "owner" {}

data "aws_region" "current" { current = true }
data "aws_caller_identity" "current" {}

data "template_file" "p" {
  template = "$${p}$${v}-$${o}"
  vars {
    v = "${replace(lower(var.version),"/[^a-zA-Z0-9]/", "")}"
    p = "${replace(lower(var.project),"/[^a-zA-Z0-9]/", "")}"
    o = "${replace(replace(lower(var.owner),"/[^a-zA-Z0-9]/", ""), "/(.{0,5})(.*)/", "$1")}"
  }
}

data "template_file" "env" {
  template = ""
  vars {
    //State machine
    "DEPLOYMENT" = "${data.template_file.p.rendered}"
  }
}
