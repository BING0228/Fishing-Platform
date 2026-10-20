import axios from "axios";
import { getStoredToken } from "@/auth/auth-store";
import { DEMO_MODE } from "@/demo/config";
import { demoAdapter } from "@/demo/mock";

const baseURL = process.env.NEXT_PUBLIC_API_BASE || "";

export const apiClient = axios.create({
  baseURL,
  timeout: 60000,
});

if (DEMO_MODE) {
  apiClient.defaults.adapter = demoAdapter;
}

apiClient.interceptors.request.use((config) => {
  const token = getStoredToken();
  if (token) {
    config.headers.Authorization = `Bearer ${token}`;
  }
  return config;
});

apiClient.interceptors.response.use(
  (response) => response,
  (error) => {
    if (error.response?.status === 401 && typeof window !== "undefined") {
      localStorage.removeItem("token");
      if (!window.location.pathname.startsWith("/login")) {
        window.location.href = "/login";
      }
    }
    return Promise.reject(error);
  },
);
