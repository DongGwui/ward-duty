/** @type {import('next').NextConfig} */
const nextConfig = {
  output: "standalone", // Dockerfile에서 .next/standalone 복사
  reactStrictMode: true,
  // /api는 동일 도메인에서 Go API가 응답 — 별도 rewrite 불필요 (Traefik이 분기)
};
export default nextConfig;
