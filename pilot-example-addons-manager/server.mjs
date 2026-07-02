import { createAppServer } from './src/server.mjs';

const server = createAppServer();
const port = Number(process.env.PORT || 5173);

server.listen(port, () => {
  console.log(`ConfigHub operational app listening on http://localhost:${port}`);
});
