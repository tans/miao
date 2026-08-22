import http from 'node:http';
import net from 'node:net';

const targetPort = Number(process.env.DSH_INTERNAL_PORT || 3080);
const server = http.createServer((request, response) => {
  const upstream = http.request({
    hostname: '127.0.0.1',
    port: targetPort,
    method: request.method,
    path: request.url,
    headers: { ...request.headers, host: `127.0.0.1:${targetPort}` }
  }, (upstreamResponse) => {
    response.writeHead(upstreamResponse.statusCode || 502, upstreamResponse.headers);
    upstreamResponse.pipe(response);
  });
  upstream.on('error', (error) => {
    if (!response.headersSent) response.writeHead(502, { 'content-type': 'text/plain; charset=utf-8' });
    response.end(`DSH upstream unavailable: ${error.message}`);
  });
  request.pipe(upstream);
});

server.on('upgrade', (request, socket, head) => {
  const upstream = net.connect(targetPort, '127.0.0.1', () => {
    upstream.write(`${request.method} ${request.url} HTTP/${request.httpVersion}\r\n`);
    for (const [name, value] of Object.entries(request.headers)) upstream.write(`${name}: ${value}\r\n`);
    upstream.write('\r\n');
    if (head.length) upstream.write(head);
    socket.pipe(upstream).pipe(socket);
  });
  upstream.on('error', () => socket.destroy());
  socket.on('error', () => upstream.destroy());
});

server.listen(Number(process.env.PORT || 8080), '0.0.0.0', () => {
  console.log(`DSH proxy listening on :${process.env.PORT || 8080}`);
});

