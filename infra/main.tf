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
    "LINE_AWS_REGION" = "${data.aws_region.current.name}"
    "LINE_AWS_ACCESS_KEY_ID" = "${aws_iam_access_key.runtime.id}"
    "LINE_AWS_SECRET_ACCESS_KEY" = "${aws_iam_access_key.runtime.secret}"
    "LINE_DEPLOYMENT" = "${data.template_file.p.rendered}"
    "LINE_RUN_ACTIVITY_ARN" = "${aws_sfn_activity.run.id}"
    "LINE_TABLE_WORKERS_NAME" = "${aws_dynamodb_table.workers.name}"
    "LINE_TABLE_WORKERS_IDX_CAP" = "${lookup(aws_dynamodb_table.workers.local_secondary_index[0], "name")}"
  }
}

output "env" {
  value = "${data.template_file.env.vars}"
}
