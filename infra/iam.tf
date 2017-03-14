resource "aws_iam_role" "lambda" {
  name = "${data.template_file.p.rendered}-lambda"
  assume_role_policy = "${data.aws_iam_policy_document.lambda_assume.json}"
}

resource "aws_iam_role_policy" "lambda" {
    name = "${data.template_file.p.rendered}-lambda"
    role = "${aws_iam_role.lambda.id}"
    policy = "${data.aws_iam_policy_document.lambda.json}"
}

data "aws_iam_policy_document" "lambda_assume" {
  policy_id = "${data.template_file.p.rendered}-lambda-assume"
  statement {
    actions = [ "sts:AssumeRole" ]
    principals {
      type = "Service"
      identifiers = ["lambda.amazonaws.com"]
    }
  }
}

data "aws_iam_policy_document" "lambda" {
  policy_id = "${data.template_file.p.rendered}-lambda"

  //allow lamba functions to write stdin/out to cloudwatch log groups
  statement {
    actions = [
      "logs:CreateLogGroup",
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams"
    ]
    resources = [
      "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${data.template_file.p.rendered}*"
    ]
  }

}