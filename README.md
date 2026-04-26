# Hister

**Your own search engine**

Hister is a general purpose web search engine providing automatic full-text indexing for visited websites.

![hister screenshot](webui/website/src/lib/assets/screenshot.png)

![hister screencast](webui/website/src/lib/assets/demo.gif)

## Features

- **Privacy-focused**: Keep your browsing history indexed locally - don't use remote search engines if it isn't necessary
- **Full-text indexing**: Search through the actual content of web pages you've visited
- **Advanced search capabilities**: Utilize a powerful [query language](https://hister.org/docs/query-language/) for precise results
- **Efficient retrieval**: Use keyword aliases to quickly find content
- **Flexible content management**: Configure blacklist and priority rules for better control
- **Local file indexing**: Index your local knowledge base
- **Crawler**: Use a (headless) browser or a traditional crawler to extend your index fast
- **Multi-user support**: Host it for your local community

## Check out our [Documentation](https://hister.org/docs/) for more details

## Development

**Requirements**: latest Go and NPM

- Clone the repository
- Build with `./manage.sh build` (or `go generate ./...; go build`)

To work on the web app with hot reload and automatic Go rebuilds:

```
npm run serve:app
```

This starts a Vite dev server (with HMR) and the Go backend (with auto-rebuild via [air](https://github.com/air-verse/air)) concurrently.

## Community

Join us on IRCNet: #hister or on [Discord](https://discord.gg/beEyuHxRSs)

## Bugs

Bugs or suggestions? Visit the [issue tracker](https://github.com/asciimoo/hister/issues).

## License

[AGPLv3](LICENSE) or any later version
