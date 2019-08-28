pipeline {
    tools {
        go 'go-1.12'
    }
    agent { docker { image 'golang' } }
    stages {
        stage('build') {
            steps {
                sh 'go version'
            }
        }
    }
}
