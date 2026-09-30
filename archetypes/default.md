---
date: '{{ .Date }}'
draft: true
title: '{{ replace .File.ContentBaseName "-" " " | title }}'
# Link-card text on Bluesky/fediverse and in feed readers (don't start it with the title)
# description: ''
# Link-card image: a file in this post's folder, or a path under assets/
# image: ''
---
