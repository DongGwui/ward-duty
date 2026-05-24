export function Spinner({ size = 4 }: { size?: number }) {
  return (
    <span
      className={`inline-block w-${size} h-${size} border-2 border-gray-300 border-t-blue-600 rounded-full animate-spin`}
      aria-label="loading"
    />
  );
}
