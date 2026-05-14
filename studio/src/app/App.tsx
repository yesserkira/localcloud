import { Routes, Route, Navigate } from 'react-router-dom';
import { Layout } from './Layout';
import { TimelinePage } from '../features/timeline/TimelinePage';
import { ScenariosPage } from '../features/scenarios/ScenariosPage';
import { ReplayRunsPage } from '../features/replay/ReplayRunsPage';
import { FaultsPage } from '../features/faults/FaultsPage';
import { ServicesPage } from '../features/services/ServicesPage';

export function App() {
  return (
    <Routes>
      <Route element={<Layout />}>
        <Route path="/" element={<Navigate to="/timeline" replace />} />
        <Route path="/timeline" element={<TimelinePage />} />
        <Route path="/scenarios" element={<ScenariosPage />} />
        <Route path="/replay/:id" element={<ReplayRunsPage />} />
        <Route path="/faults" element={<FaultsPage />} />
        <Route path="/services" element={<ServicesPage />} />
      </Route>
    </Routes>
  );
}
