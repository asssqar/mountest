import { api } from "../api/client";
import type {
  AdminMe,
  AdminPasswordReset,
  AdminUser,
  AdminUserCreated,
  Question,
  Subject,
  Variant,
} from "../api/types";

export const adminApi = {
  me: () => api.get<AdminMe>("/admin/me"),
  login: (username: string, password: string) =>
    api.post<AdminMe>("/admin/login", { username, password }),
  logout: () => api.post<void>("/admin/logout"),

  listSubjects: () => api.get<Subject[]>("/admin/subjects"),
  createSubject: (name: string) => api.post<{ id: string }>("/admin/subjects", { name }),
  updateSubject: (id: string, name: string) =>
    api.put<{ id: string }>(`/admin/subjects/${id}`, { name }),
  deleteSubject: (id: string) => api.delete<void>(`/admin/subjects/${id}`),

  listVariants: (subjectId?: string) => {
    const q = subjectId ? `?subjectId=${subjectId}` : "";
    return api.get<Variant[]>(`/admin/variants${q}`);
  },
  createVariant: (input: { subjectId: string; title: string; durationMinutes: number }) =>
    api.post<{ id: string }>("/admin/variants", input),
  updateVariant: (
    id: string,
    input: { subjectId: string; title: string; durationMinutes: number },
  ) => api.put<{ id: string }>(`/admin/variants/${id}`, input),
  deleteVariant: (id: string) => api.delete<void>(`/admin/variants/${id}`),

  listQuestions: (variantId: string) =>
    api.get<Question[]>(`/admin/questions?variantId=${variantId}`),
  getQuestion: (id: string) => api.get<Question>(`/admin/questions/${id}`),
  createQuestion: (input: {
    variantId: string;
    text: string;
    position?: number;
    options: { text: string; isCorrect: boolean }[];
  }) => api.post<{ id: string }>("/admin/questions", input),
  updateQuestion: (
    id: string,
    input: {
      variantId: string;
      text: string;
      position: number;
      options: { id?: string; text: string; isCorrect: boolean }[];
    },
  ) => api.put<{ id: string }>(`/admin/questions/${id}`, input),
  deleteQuestion: (id: string) => api.delete<void>(`/admin/questions/${id}`),

  // ---- users (только superadmin) ----
  listUsers: () => api.get<AdminUser[]>("/admin/users"),
  createUser: (email: string) =>
    api.post<AdminUserCreated>("/admin/users", { email }),
  resetUserPassword: (id: string) =>
    api.post<AdminPasswordReset>(`/admin/users/${id}/reset-password`),
  deleteUser: (id: string) => api.delete<void>(`/admin/users/${id}`),
};
