resource "aws_cloudwatch_log_group" "gateway" {
    name = "/aws/lambda/${aws_lambda_function.gateway.function_name}"
    retention_in_days = 60
}

resource "aws_cloudwatch_log_group" "alloc" {
    name = "/aws/lambda/${aws_lambda_function.alloc.function_name}"
    retention_in_days = 60
}

resource "aws_cloudwatch_event_rule" "alloc_tick" {
  name        = "${data.template_file.p.rendered}-alloc-tick"
  description = "Dispatch run activities to the correct pool"
  schedule_expression = "rate(1 minute)"
}

resource "aws_cloudwatch_event_target" "sns" {
  rule      = "${aws_cloudwatch_event_rule.alloc_tick.name}"
  target_id = "${data.template_file.p.rendered}-alloc-func"
  arn       = "${aws_lambda_function.alloc.arn}"
}
