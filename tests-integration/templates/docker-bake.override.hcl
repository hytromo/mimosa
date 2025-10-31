target "target1"{
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}

{% if targets == 'multiple' %}
target "target2"{
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
}
{% endif %}