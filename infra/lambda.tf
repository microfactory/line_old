resource "aws_lambda_function" "alloc" {
  function_name = "${data.template_file.p.rendered}-alloc"
  description = "claims worker capacity for a task"
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

resource "aws_lambda_function" "dealloc" {
  function_name = "${data.template_file.p.rendered}-dealloc"
  description = "releases worker capacity for a task"
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

resource "aws_lambda_function" "dispatch" {
  function_name = "${data.template_file.p.rendered}-dispatch"
  description = "send a scheduled task to the worker's queue"
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
