data "template_file" "machine" {
    template = "${file("${path.module}/machine.json")}"
    vars {
      run_act_arn = "${aws_sfn_activity.run.id}"
    }
}

resource "aws_sfn_activity" "run" {
  name = "${data.template_file.p.rendered}-run"
}

resource "aws_sfn_state_machine" "schedule" {
  name     = "${data.template_file.p.rendered}-schedule"
  role_arn = "${aws_iam_role.lambda.arn}"
  definition = "${data.template_file.machine.rendered}"
}
