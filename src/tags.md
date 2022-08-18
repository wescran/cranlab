---
title: 'Tag Archive'
layout: 'layouts/feed.njk'
pagination:
  data: collections
  size: 1
  alias: tag
  filter: ['all', 'nav', 'post', 'posts', 'tagList', 'rss']
permalink: '/tags/{{ tag | slug }}/'
---