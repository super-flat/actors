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

add-code:
	FROM +install-deps
	# add code
	COPY --dir app .

mock:
	FROM +add-code
	RUN mockery --all --keeptree
	SAVE ARTIFACT ./mocks AS LOCAL mocks

vendor:
	FROM +mock
	RUN --ssh go mod vendor
