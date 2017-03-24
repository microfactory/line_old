resource "aws_dynamodb_table" "pools" {
  name = "${data.template_file.p.rendered}-pools"
  read_capacity = 1
  write_capacity = 1
  hash_key = "pool"

  // stream_enabled = true
  // stream_view_type = "NEW_IMAGE"

  attribute {
    name = "pool" //pool id
    type = "S"
  }
}
