resource "aws_sqs_queue" "schedule" {
  name                      = "${data.template_file.p.rendered}-schedule"
  delay_seconds             = 0
  max_message_size          = 2048
  message_retention_seconds = 1209600
  receive_wait_time_seconds = 20
}
