pipeline {
    tools {
        go 'Go installer'
        docker 'My Docker'  
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
