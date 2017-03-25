resource "aws_dynamodb_table" "pools" {
  name = "${data.template_file.p.rendered}-pools"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"

  attribute {
    name = "pool" //pool id
    type = "S"
  }
}

resource "aws_dynamodb_table" "tasks" {
  name = "${data.template_file.p.rendered}-tasks"
  read_capacity = 1
  write_capacity = 1
  hash_key = "prj"
  range_key = "tsk"

  attribute {
    name = "prj" //project id
    type = "S"
  }

  attribute {
    name = "tsk" //task id
    type = "S"
  }

  attribute {
    name = "pool" //pool id
    type = "S"
  }

  global_secondary_index {
    name               = "pool_idx"
    hash_key           = "pool"
    range_key          = "tsk"
    write_capacity     = 1
    read_capacity      = 1
    projection_type    = "KEYS_ONLY"
  }
}
