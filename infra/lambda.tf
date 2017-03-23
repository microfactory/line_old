resource "aws_lambda_function" "dispatch" {
  function_name = "${data.template_file.p.rendered}-dispatch"
  description = "send task to the pool's queue"
  filename = "handler.zip"
  source_code_hash = "${base64sha256(file("handler.zip"))}"
  role = "${aws_iam_role.lambda.arn}"

  timeout = "65"
  memory_size = "128"
  handler = "handler.Handle"
  runtime = "python2.7"
  environment = {
    variables = "${data.template_file.env.vars}"
  }
}

resource "aws_lambda_permission" "allow_event" {
  statement_id = "${data.template_file.p.rendered}-event"
  action = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.dispatch.arn}"
  principal = "events.amazonaws.com"
  source_arn = "${aws_cloudwatch_event_rule.dispatch_tick.arn}"
}

resource "aws_lambda_function" "gateway" {
  function_name = "${data.template_file.p.rendered}-gateway"
  description = "handles http requests from the API gateway"
  filename = "handler.zip"
  source_code_hash = "${base64sha256(file("handler.zip"))}"
  role = "${aws_iam_role.lambda.arn}"

  timeout = "10"
  memory_size = "128"
  handler = "handler.Handle"
  runtime = "python2.7"
  environment = {
    variables = "${data.template_file.env.vars}"
  }
}

resource "aws_lambda_permission" "allow_gateway" {
  statement_id = "AllowExecutionFromGateway"
  action = "lambda:InvokeFunction"
  function_name = "${aws_lambda_function.gateway.arn}"
  principal = "apigateway.amazonaws.com"
  source_arn = "arn:aws:execute-api:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:${aws_api_gateway_rest_api.main.id}/*/*/*"
}
