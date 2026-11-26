import axios from "axios";

// The page and the API share one origin, so requests are relative. Upstream's
// CSRF token, session cookies and auth monitoring belonged to KFStats' accounts;
// KeyFordle has none, and the server rejects cross-origin POSTs instead.
const api = axios.create({
  timeout: 10000,
  headers: {
    "Content-Type": "application/json",
  },
});

export default api;
