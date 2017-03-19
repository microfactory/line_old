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
      identifiers = [
        "lambda.amazonaws.com",
        "states.${data.aws_region.current.name}.amazonaws.com"
      ]
    }
  }
}

data "aws_iam_policy_document" "lambda" {
  policy_id = "${data.template_file.p.rendered}-lambda"

  //allow role to invoke lambda functions
  statement {
    actions = ["lambda:InvokeFunction"]
    resources = [
      "arn:aws:lambda:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:function:line001-advan-*",
    ]
  }

  //allow role to write stdin/out to cloudwatch log groups
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

  statement {
    actions = [
      "dynamodb:GetItem",
      "dynamodb:PutItem",
      "dynamodb:UpdateItem",
      "dynamodb:DeleteItem",
      "dynamodb:Query"
    ]

    resources = ["${aws_dynamodb_table.workers.arn}*"]
  }

  //allow state machines activities to be informed
  statement {
    actions = [
      "states:SendTaskHeartbeat",
      "states:SendTaskFailure",
      "states:SendTaskSuccess",
      "states:StartExecution",
      "states:StopExecution",
      "states:GetActivityTask"
    ]
    resources = [
      "${aws_sfn_activity.run.id}",
      "${aws_sfn_state_machine.schedule.id}"
    ]
  }

}
