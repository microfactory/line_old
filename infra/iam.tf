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
      "arn:aws:lambda:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:function:${data.template_file.p.rendered}-*",
    ]
  }

  statement {
    actions = ["states:StartExecution"]
    resources = ["*"]
  }

  //allow role to write stdin/out to cloudwatch log groups
  statement {
    actions = [
      "logs:CreateLogStream",
      "logs:PutLogEvents",
      "logs:DescribeLogStreams"
    ]
    resources = [
      "arn:aws:logs:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:log-group:/aws/lambda/${data.template_file.p.rendered}*"
    ]
  }
}

//a user of which credentials are carefully scoped for runtime resource management and handing out federated tokens
resource "aws_iam_user" "runtime" {
  force_destroy = true
  name = "${data.template_file.p.rendered}-runtime"
  path = "/${data.template_file.p.rendered}/"
}

resource "aws_iam_user_policy" "runtime" {
  name = "${data.template_file.p.rendered}-runtime"
  user = "${aws_iam_user.runtime.name}"
  policy = "${data.aws_iam_policy_document.runtime.json}"
}

data "aws_iam_policy_document" "runtime" {
  policy_id = "${data.template_file.p.rendered}-runtime"

  //allow state machines activities to be informed
  statement {
    actions = [
      "states:SendTaskHeartbeat",
      "states:SendTaskFailure",
      "states:SendTaskSuccess",
      "states:StartExecution",
      "states:StopExecution",
      "states:GetActivityTask",
      "states:ListExecutions",
      "states:GetExecutionHistory"
    ]

    resources = [
      "${aws_sfn_activity.run.id}",
      "${aws_sfn_state_machine.schedule.id}",
      "arn:aws:states:${data.aws_region.current.name}:${data.aws_caller_identity.current.account_id}:execution:${aws_sfn_state_machine.schedule.name}:*"
    ]
  }

  statement {
    actions = [
      "sqs:SendMessage",
      "sqs:CreateQueue",
      "sqs:ReceiveMessage",
      "sqs:DeleteQueue",
      "sqs:DeleteMessage"
    ]
    resources = [
      "arn:aws:sqs:*:${data.aws_caller_identity.current.account_id}:${data.template_file.p.rendered}*"
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
    resources = [
      "${aws_dynamodb_table.pools.arn}*",
    ]
  }

}

resource "aws_iam_access_key" "runtime" {
  user    = "${aws_iam_user.runtime.name}"
}
