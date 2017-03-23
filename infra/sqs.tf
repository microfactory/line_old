resource "aws_sqs_queue" "pool" {
  name                      = "${data.template_file.p.rendered}-pool"
  message_retention_seconds = 60
  receive_wait_time_seconds = 10
}
