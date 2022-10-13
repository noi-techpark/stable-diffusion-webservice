FROM golang:1.18-bullseye

WORKDIR app

COPY noi-sd-ws noi-sd-ws
COPY queue.db queue.db

CMD [ "./noi-sd-ws" , "queue.db"]