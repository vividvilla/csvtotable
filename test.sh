pip freeze
nosetests --with-cov --cover-package excel2table --cover-package tests && flake8 . --exclude=.moban.d --builtins=unicode,xrange,long
