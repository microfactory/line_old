resource "aws_lambda_function" "core" {
  function_name = "${data.template_file.p.rendered}-core"
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
