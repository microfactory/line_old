variable "project" {}
variable "version" {}
variable "owner" {}
resource "random_id" "id" { byte_length = 4 }

data "aws_region" "current" { current = true }
data "aws_caller_identity" "current" {}

data "template_file" "p" {
  template = "$${i}-$${p}$${v}-$${o}"
  vars {
    i = "${random_id.id.hex}"
    v = "${replace(lower(var.version),"/[^a-zA-Z0-9]/", "")}"
    p = "${replace(lower(var.project),"/[^a-zA-Z0-9]/", "")}"
    o = "${replace(replace(lower(var.owner),"/[^a-zA-Z0-9]/", ""), "/(.{0,5})(.*)/", "$1")}"
  }
}

data "template_file" "env" {
  template = ""
  vars {
    "LINE_POOL_QUEUE_URL" = "${aws_sqs_queue.pool.id}"
    "LINE_STATE_MACHINE_ARN" = "arn:aws:states:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:stateMachine:${data.template_file.p.rendered}-schedule"
    "LINE_AWS_REGION" = "${data.aws_region.current.name}"
    "LINE_AWS_ACCESS_KEY_ID" = "${aws_iam_access_key.runtime.id}"
    "LINE_AWS_SECRET_ACCESS_KEY" = "${aws_iam_access_key.runtime.secret}"
    "LINE_DEPLOYMENT" = "${data.template_file.p.rendered}"
    "LINE_RUN_ACTIVITY_ARN" = "${aws_sfn_activity.run.id}"
  }
}

output "env" {
  value = "${data.template_file.env.vars}"
}
