'use strict';

module.exports = function (grunt) {
    require('load-grunt-config')(grunt);

    grunt.initConfig({
        watch: {
            test: {
                files: ['Gruntfile.js', '**/*.go'],
                tasks: ['shell:test'],
                options: {
                    atBegin: true
                }
            }
        },
        shell: {
            test: {
                command: 'go test -v github.com/peferron/alternate/...'
            },
            install: {
                command: 'go install github.com/peferron/alternate'
            }
        }
    });

    grunt.registerTask('test', [
        'watch:test'
    ]);

    grunt.registerTask('install', [
        'shell:test',
        'shell:install'
    ]);
};
