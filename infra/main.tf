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
    "LINE_DEPLOYMENT" = "${data.template_file.p.rendered}"
    "LINE_AWS_ACCOUNT_ID" = "${data.aws_caller_identity.current.account_id}"
    "LINE_AWS_REGION" = "${data.aws_region.current.name}"
    "LINE_AWS_ACCESS_KEY_ID" = "${aws_iam_access_key.runtime.id}"
    "LINE_AWS_SECRET_ACCESS_KEY" = "${aws_iam_access_key.runtime.secret}"

    "LINE_ALLOC_TTL" = "30"
    "LINE_MAX_RETRY" = "3"

    "LINE_SCHEDULE_QUEUE_URL" = "${aws_sqs_queue.schedule.id}"
    "LINE_SCHEDULE_DLQUEUE_URL" = "${aws_sqs_queue.schedule_dlq.id}"

    "LINE_TABLE_NAME_POOLS" = "${aws_dynamodb_table.pools.name}"
    "LINE_TABLE_NAME_REPLICAS" = "${aws_dynamodb_table.replicas.name}"
    "LINE_TABLE_NAME_WORKERS" = "${aws_dynamodb_table.workers.name}"
    "LINE_TABLE_IDX_WORKERS_CAP" = "${lookup(aws_dynamodb_table.workers.global_secondary_index[0], "name")}"
    "LINE_TABLE_IDX_ALLOCS_TTL" = "${lookup(aws_dynamodb_table.allocs.local_secondary_index[0], "name")}"
    "LINE_TABLE_NAME_ALLOCS" = "${aws_dynamodb_table.allocs.name}"
  }
}

output "env" {
  value = "${data.template_file.env.vars}"
}

output "endpoint" {
  value = "https://${aws_api_gateway_rest_api.main.id}.execute-api.eu-west-1.amazonaws.com/${aws_api_gateway_deployment.main.stage_name}"
}
