pipeline {
    tools {
        go 'Go installer'
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
