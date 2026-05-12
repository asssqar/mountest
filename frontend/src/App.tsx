import { Route, Routes } from "react-router-dom";
import Layout from "./components/Layout";
import HomePage from "./pages/HomePage";
import SubjectPage from "./pages/SubjectPage";
import AttemptPage from "./pages/AttemptPage";
import ResultPage from "./pages/ResultPage";
import AdminLayout from "./admin/AdminLayout";
import AdminLoginPage from "./admin/AdminLoginPage";
import AdminSubjectsPage from "./admin/AdminSubjectsPage";
import AdminVariantsPage from "./admin/AdminVariantsPage";
import AdminQuestionsPage from "./admin/AdminQuestionsPage";
import AdminUsersPage from "./admin/AdminUsersPage";

export default function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route index element={<HomePage />} />
        <Route path="subjects/:id" element={<SubjectPage />} />
        <Route path="attempts/:id" element={<AttemptPage />} />
        <Route path="attempts/:id/result" element={<ResultPage />} />

        <Route path="admin" element={<AdminLayout />}>
          <Route path="login" element={<AdminLoginPage />} />
          <Route index element={<AdminSubjectsPage />} />
          <Route path="variants" element={<AdminVariantsPage />} />
          <Route path="questions" element={<AdminQuestionsPage />} />
          <Route path="users" element={<AdminUsersPage />} />
        </Route>
      </Route>
    </Routes>
  );
}
