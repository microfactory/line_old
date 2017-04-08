resource "aws_cloudwatch_log_group" "gateway" {
    name = "/aws/lambda/${aws_lambda_function.gateway.function_name}"
    retention_in_days = 60
}

resource "aws_cloudwatch_log_group" "schedule" {
    name = "/aws/lambda/${aws_lambda_function.schedule.function_name}"
    retention_in_days = 60
}

resource "aws_cloudwatch_log_group" "release" {
    name = "/aws/lambda/${aws_lambda_function.release.function_name}"
    retention_in_days = 60
}

//
// Round based logic is called periodically 
//

resource "aws_cloudwatch_event_rule" "round_tick" {
  name        = "${data.template_file.p.rendered}-round-tick"
  description = "fires an allocate and release cycle"
  schedule_expression = "rate(1 minute)"
}

resource "aws_cloudwatch_event_target" "schedule" {
  rule      = "${aws_cloudwatch_event_rule.round_tick.name}"
  target_id = "${data.template_file.p.rendered}-schedule-func"
  arn       = "${aws_lambda_function.schedule.arn}"
}

resource "aws_cloudwatch_event_target" "release" {
  rule      = "${aws_cloudwatch_event_rule.round_tick.name}"
  target_id = "${data.template_file.p.rendered}-release-func"
  arn       = "${aws_lambda_function.release.arn}"
}
