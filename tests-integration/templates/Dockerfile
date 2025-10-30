FROM alpine:latest AS target1

RUN echo hi_target1 >/tmp/a

ADD . /added_files_target1/

{% if targets == 'multiple' %}
FROM alpine:latest AS target2

COPY --from=target1 /tmp/a /tmp/a

RUN echo hi_target2 >/tmp/b

ADD . /added_files_target2/
{% endif %}