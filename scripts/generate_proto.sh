mkdir -p plugin/proto/src/golang
protoc --go_out=plugin/proto/src/golang/aws --go_opt=paths=source_relative \
    --go-grpc_out=plugin/proto/src/golang/aws --go-grpc_opt=paths=source_relative \
    plugin/proto/*.proto
mv plugin/proto/src/golang/aws/plugin/proto/* plugin/proto/src/golang/aws/
rm -rf plugin/proto/src/golang/aws/plugin