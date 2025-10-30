variable "TARGET1_TAGS" {
  type = list(string)
}

{% if targets == 'multiple' %}
variable "TARGET2_TAGS" {
  type = list(string)
}
{% endif %}

target "target1"{

  target = "target1"

  {% if dockerfile_type == 'single' %}
    {% if dockerfile_location == 'root' %}
  dockerfile = "Dockerfile"
    {% else %}
  dockerfile = "subdir/Dockerfile"
    {% endif %}
  {% else %}
    {% if dockerfile_location == 'root' %}
  dockerfile = "Dockerfile.target1"
    {% else %}
  dockerfile = "subdir/Dockerfile.target1"
    {% endif %}
  {% endif %}

  tags = [for tag in TARGET1_TAGS : tag]

{# if we have multiple bakefiles, the 2nd one will include the platforms field #}
{% if bakefile_type == 'single' %}
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
{% endif %}
  context = "."
}

{% if targets == 'multiple' %}
target "target2"{

  target = "target2"

  {% if dockerfile_type == 'single' %}
    {% if dockerfile_location == 'root' %}
  dockerfile = "Dockerfile"
    {% else %}
  dockerfile = "subdir/Dockerfile"
    {% endif %}
  {% else %}
    {% if dockerfile_location == 'root' %}
  dockerfile = "Dockerfile.target2"
    {% else %}
  dockerfile = "subdir/Dockerfile.target2"
    {% endif %}
  {% endif %}

  tags = [for tag in TARGET2_TAGS : tag]

  {% if bakefile_type == 'single' %}
  platforms = [
    "linux/amd64",
    "linux/arm64"
  ]
  {% endif %}
  context = "."
}
{% endif %}

group "default" {
  targets = [
    "target1",
{% if targets == 'multiple' %}
    "target2",
{% endif %}
  ]
}