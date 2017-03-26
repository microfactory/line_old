resource "aws_dynamodb_table" "workers" {
  name = "${data.template_file.p.rendered}-workers"
  read_capacity = 1
  write_capacity = 1
  hash_key = "wrk"

  attribute {
    name = "wrk"
    type = "S"
  }

  attribute {
    name = "cap"
    type = "N"
  }

  global_secondary_index {
    name               = "cap_idx"
    hash_key           = "wrk"
    range_key          = "cap"
    projection_type    = "INCLUDE"
    write_capacity     = 1
    read_capacity      = 1
    non_key_attributes = ["cap"]
  }
}

resource "aws_dynamodb_table" "tasks" {
  name = "${data.template_file.p.rendered}-tasks"
  read_capacity = 1
  write_capacity = 1
  hash_key = "tsk"

  attribute {
    name = "tsk"
    type = "S"
  }
}

resource "aws_dynamodb_table" "allocs" {
  name = "${data.template_file.p.rendered}-allocs"
  read_capacity = 1
  write_capacity = 1
  hash_key = "wrk"
  range_key = "tsk"

  attribute {
    name = "tsk"
    type = "S"
  }

  attribute {
    name = "wrk"
    type = "S"
  }

}
