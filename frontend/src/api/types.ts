export type Subject = {
  id: string;
  name: string;
  variantsCount: number;
};

export type Variant = {
  id: string;
  subjectId: string;
  title: string;
  /** Необязательная тема — показывается под названием теста */
  topic?: string | null;
  durationMinutes: number;
  questionsCount: number;
  /** Опубликован ли вариант. Поле приходит только из админских ручек. */
  isPublished?: boolean;
  /** Только у superadmin в списке вариантов: логин/email автора */
  createdByUsername?: string | null;
};

export type AdminRole = "superadmin" | "editor";

export type AdminMe = {
  username: string;
  role: AdminRole;
};

export type AdminUser = {
  id: string;
  username: string;
  role: AdminRole;
  createdAt: string;
};

export type AdminUserCreated = AdminUser & {
  password: string;
};

export type AdminPasswordReset = {
  id: string;
  password: string;
};

export type AnswerOption = {
  id: string;
  text: string;
  position: number;
  isCorrect: boolean;
};

export type Question = {
  id: string;
  variantId: string;
  position: number;
  text: string;
  imageUrl?: string | null;
  options: AnswerOption[];
};

export type Guest = {
  id: string;
  firstName: string;
  lastName: string;
};

export type Attempt = {
  id: string;
  variantId: string;
  variantTitle: string;
  subjectName: string;
  durationMinutes: number;
  startedAt: string;
  finishedAt: string | null;
  questions: Question[];
  answers: Record<string, string[]>;
  guest?: Guest;
  /** Возвращается ТОЛЬКО на POST /attempts. Дальше клиент сам шлёт его в X-Attempt-Token. */
  attemptToken?: string;
};

export type ReviewStatus = "correct" | "incorrect" | "unanswered";

/** Один вопрос на экране результата — со статусом, выбранными и правильными опциями. */
export type ReviewEntry = {
  questionId: string;
  position: number;
  questionText: string;
  questionImageUrl?: string | null;
  options: AnswerOption[];
  selectedOptionIds: string[];
  correctOptionIds: string[];
  status: ReviewStatus;
};

export type AttemptResult = {
  attemptId: string;
  score: number;
  total: number;
  startedAt: string;
  finishedAt: string;
  /** Все вопросы попытки в порядке. Фронт сам решает, фильтровать ли по статусу. */
  review: ReviewEntry[];
  guest?: Guest;
};
