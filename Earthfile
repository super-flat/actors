FROM ghcr.io/troop-dev/go-kit:1.17.0

install-deps:
	# add git to known hosts
	RUN mkdir -p /root/.ssh && \
		chmod 700 /root/.ssh && \
		ssh-keyscan github.com >> /root/.ssh/known_hosts
	RUN git config --global url."git@github.com:".insteadOf "https://github.com/"
	# add dependencies
	COPY go.mod go.sum .
	RUN --ssh go mod download -x

protogen:
	FROM +install-deps
	# copy the proto files to generate
	COPY --dir proto/ ./
	COPY buf.work.yaml buf.gen.yaml ./
	RUN ls -la proto/local/actors/v1
	# generate the pbs
	RUN buf generate \
		--template buf.gen.yaml \
		--path proto/local/actors/v1
	# save artifact to
	SAVE ARTIFACT gen gen AS LOCAL gen

add-code:
	FROM +protogen
	# add code
	COPY --dir app .

mock:
	FROM +add-code
	RUN mockery --all --keeptree
	SAVE ARTIFACT ./mocks AS LOCAL mocks

vendor:
	FROM +mock
	RUN --ssh go mod vendor
