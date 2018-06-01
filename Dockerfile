FROM golang:1.10.2

# Create the directories needed
RUN mkdir -p /underdarkgo
RUN mkdir -p /underdarkgo/data

# Copy the production files
ADD underdarkgo /underdarkgo/underdarkgo
ADD assets /underdarkgo/assets

WORKDIR /underdarkgo

EXPOSE 8081

ENTRYPOINT ["/underdarkgo/underdarkgo", "/underdarkgo/data"]
