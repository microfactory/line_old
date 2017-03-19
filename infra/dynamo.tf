resource "aws_dynamodb_table" "workers" {
  name = "${data.template_file.p.rendered}-workers"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"
  range_key = "que"

  attribute {
    name = "pool" //pool id
    type = "S"
  }

  attribute {
    name = "que" //queue url
    type = "S"
  }

  attribute {
    name = "cap" //capacity
    type = "N"
  }

  local_secondary_index {
    name = "cap_idx"
    range_key = "cap"
    projection_type = "KEYS_ONLY"
  }
}
