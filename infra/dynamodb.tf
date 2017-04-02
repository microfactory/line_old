resource "aws_dynamodb_table" "pools" {
  name = "${data.template_file.p.rendered}-pools"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"

  attribute {
    name = "pool"
    type = "S"
  }
}

resource "aws_dynamodb_table" "replicas" {
  name = "${data.template_file.p.rendered}-replicas"
  read_capacity = 1
  write_capacity = 1
  hash_key = "set"
  range_key = "pwrk"

  attribute {
    name = "pwrk" //pool:worker
    type = "S"
  }

  attribute {
    name = "set"  //dataset
    type = "S"
  }
}

resource "aws_dynamodb_table" "workers" {
  name = "${data.template_file.p.rendered}-workers"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"
  range_key = "wrk"

  attribute {
    name = "pool"
    type = "S"
  }

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
    hash_key           = "pool"
    range_key          = "cap"
    projection_type    = "KEYS_ONLY"
    write_capacity     = 1
    read_capacity      = 1
  }
}

resource "aws_dynamodb_table" "allocs" {
  name = "${data.template_file.p.rendered}-allocs"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"
  range_key = "alloc"

  attribute {
    name = "pool"
    type = "S"
  }

  attribute {
    name = "alloc"
    type = "S"
  }

  attribute {
    name = "ttl"
    type = "N"
  }

  local_secondary_index {
    name               = "ttl_idx"
    range_key          = "ttl"
    projection_type    = "INCLUDE"
    non_key_attributes = ["wrk", "eval"]
  }
}
