export const DEMO_MODE = process.env.NEXT_PUBLIC_DEMO_MODE === "1";
export const DEMO_TOKEN = "demo-session";
export const DEMO_USER = {
  id: 1,
  username: "demo",
  role: "admin" as const,
};
