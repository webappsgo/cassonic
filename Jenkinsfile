pipeline {
    agent any

    environment {
        CGO_ENABLED = '0'
        REGISTRY = "ghcr.io/${env.GIT_URL.tokenize('/')[3]}/${env.GIT_URL.tokenize('/')[4].replaceAll('\\.git$', '')}"
    }

    stages {
        stage('Checkout') {
            steps {
                checkout scm
            }
        }

        stage('Build Info') {
            steps {
                script {
                    def releaseFile = fileExists('release.txt') ? readFile('release.txt').trim() : '0.1.0'
                    env.VERSION = releaseFile
                    env.COMMIT_ID = sh(script: 'git rev-parse --short HEAD', returnStdout: true).trim()
                    env.BUILD_DATE = sh(script: 'date -u +"%Y-%m-%dT%H:%M:%SZ"', returnStdout: true).trim()
                }
            }
        }

        stage('Build') {
            agent {
                docker {
                    image "ghcr.io/${env.GIT_URL.tokenize('/')[3]}/${env.GIT_URL.tokenize('/')[4].replaceAll('\\.git$', '')}:build"
                    reuseNode true
                }
            }
            steps {
                sh '''
                    LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildDate=${BUILD_DATE}'"
                    go build -ldflags "$LDFLAGS" -o /dev/null ./src
                    go build -ldflags "$LDFLAGS" -o /dev/null ./src/client
                '''
            }
        }

        stage('Test') {
            agent {
                docker {
                    image "ghcr.io/${env.GIT_URL.tokenize('/')[3]}/${env.GIT_URL.tokenize('/')[4].replaceAll('\\.git$', '')}:build"
                    reuseNode true
                }
            }
            steps {
                sh 'go vet ./...'
                sh 'go test -cover -coverprofile=coverage.out ./...'
            }
        }

        stage('Security') {
            parallel {
                stage('Vuln Scan') {
                    agent {
                        docker {
                            image "ghcr.io/${env.GIT_URL.tokenize('/')[3]}/${env.GIT_URL.tokenize('/')[4].replaceAll('\\.git$', '')}:build"
                            reuseNode true
                        }
                    }
                    steps {
                        sh 'test -f go.sum && govulncheck ./... || echo "No go.sum, skipping"'
                    }
                }
            }
        }

        stage('Release') {
            when {
                tag 'v*'
            }
            agent {
                docker {
                    image "ghcr.io/${env.GIT_URL.tokenize('/')[3]}/${env.GIT_URL.tokenize('/')[4].replaceAll('\\.git$', '')}:build"
                    reuseNode true
                }
            }
            steps {
                sh '''
                    mkdir -p binaries
                    LDFLAGS="-s -w -X 'main.Version=${VERSION}' -X 'main.CommitID=${COMMIT_ID}' -X 'main.BuildDate=${BUILD_DATE}'"
                    for platform in linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 windows/amd64 windows/arm64 freebsd/amd64 freebsd/arm64; do
                        OS=${platform%/*}
                        ARCH=${platform#*/}
                        OUTPUT="binaries/cassonic-${OS}-${ARCH}"
                        [ "$OS" = "windows" ] && OUTPUT="${OUTPUT}.exe"
                        GOOS=$OS GOARCH=$ARCH go build -ldflags "$LDFLAGS" -o "$OUTPUT" ./src
                        OUTPUT_CLI="binaries/cassonic-cli-${OS}-${ARCH}"
                        [ "$OS" = "windows" ] && OUTPUT_CLI="${OUTPUT_CLI}.exe"
                        GOOS=$OS GOARCH=$ARCH go build -ldflags "$LDFLAGS" -o "$OUTPUT_CLI" ./src/client
                    done
                    cd binaries && sha256sum * > SHA256SUMS.txt
                '''
                archiveArtifacts artifacts: 'binaries/*', fingerprint: true
            }
        }
    }

    post {
        always {
            cleanWs()
        }
    }
}
