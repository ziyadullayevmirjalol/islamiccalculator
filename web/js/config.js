// API base URL: defaults to the local backend; override at runtime with
//   localStorage.setItem('ic.apiBase', 'https://staging.example.com')
export const API_BASE =
  localStorage.getItem('ic.apiBase') || 'http://localhost:8080';
