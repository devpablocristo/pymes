import React, { lazy, Suspense, Component } from "react";
import { Route, Routes } from "react-router-dom";
import RouteScrollToTop from "./helper/RouteScrollToTop";

class PageErrorBoundary extends Component {
  constructor(props) {
    super(props);
    this.state = { error: null };
  }
  static getDerivedStateFromError(error) {
    return { error };
  }
  render() {
    if (this.state.error) {
      return (
        <div style={{ padding: '2rem', fontFamily: 'system-ui' }}>
          <h2 style={{ color: '#ef4444' }}>Error en esta página</h2>
          <pre style={{ whiteSpace: 'pre-wrap', color: '#f87171', fontSize: '0.82rem', margin: '1rem 0', maxHeight: '200px', overflow: 'auto', background: '#1e1e1e', padding: '1rem', borderRadius: '8px' }}>
            {this.state.error.message}
          </pre>
          <button
            onClick={() => this.setState({ error: null })}
            style={{ marginTop: '1rem', padding: '0.5rem 1rem', cursor: 'pointer' }}
          >
            Reintentar
          </button>
        </div>
      );
    }
    return this.props.children;
  }
}

const Loader = <div style={{ padding: '2rem' }}>Cargando…</div>;

function W({ C }) {
  return (
    <PageErrorBoundary>
      <Suspense fallback={Loader}>
        <C />
      </Suspense>
    </PageErrorBoundary>
  );
}

// Lazy imports explícitos

const AddUserPage = lazy(() => import("./pages/AddUserPage"));
const UsersListPage = lazy(() => import("./pages/UsersListPage"));
const UsersGridPage = lazy(() => import("./pages/UsersGridPage"));
const ViewProfilePage = lazy(() => import("./pages/ViewProfilePage"));
const AssignRolePage = lazy(() => import("./pages/AssignRolePage"));
const RoleAccessPage = lazy(() => import("./pages/RoleAccessPage"));


const FormPage = lazy(() => import("./pages/FormPage"));
const FormLayoutPage = lazy(() => import("./pages/FormLayoutPage"));
const FormValidationPage = lazy(() => import("./pages/FormValidationPage"));
const WizardPage = lazy(() => import("./pages/WizardPage"));
const ImageUploadPage = lazy(() => import("./pages/ImageUploadPage"));

const TableBasicPage = lazy(() => import("./pages/TableBasicPage"));
const TableDataPage = lazy(() => import("./pages/TableDataPage"));

const LineChartPage = lazy(() => import("./pages/LineChartPage"));
const ColumnChartPage = lazy(() => import("./pages/ColumnChartPage"));
const PieChartPage = lazy(() => import("./pages/PieChartPage"));
const WidgetsPage = lazy(() => import("./pages/WidgetsPage"));

const EmailPage = lazy(() => import("./pages/EmailPage"));
const StarredPage = lazy(() => import("./pages/StarredPage"));
const ChatEmptyPage = lazy(() => import("./pages/ChatEmptyPage"));
const ChatMessagePage = lazy(() => import("./pages/ChatMessagePage"));
const ChatProfilePage = lazy(() => import("./pages/ChatProfilePage"));


const TextGeneratorPage = lazy(() => import("./pages/TextGeneratorPage"));
const TextGeneratorNewPage = lazy(() => import("./pages/TextGeneratorNewPage"));
const CodeGeneratorPage = lazy(() => import("./pages/CodeGeneratorPage"));
const CodeGeneratorNewPage = lazy(() => import("./pages/CodeGeneratorNewPage"));
const ImageGeneratorPage = lazy(() => import("./pages/ImageGeneratorPage"));
const VoiceGeneratorPage = lazy(() => import("./pages/VoiceGeneratorPage"));
const VideoGeneratorPage = lazy(() => import("./pages/VideoGeneratorPage"));

const WalletPage = lazy(() => import("./pages/WalletPage"));
const MarketplacePage = lazy(() => import("./pages/MarketplacePage"));
const MarketplaceDetailsPage = lazy(() => import("./pages/MarketplaceDetailsPage"));
const PortfolioPage = lazy(() => import("./pages/PortfolioPage"));

const GalleryPage = lazy(() => import("./pages/GalleryPage"));
const GalleryGridPage = lazy(() => import("./pages/GalleryGridPage"));
const GalleryMasonryPage = lazy(() => import("./pages/GalleryMasonryPage"));
const GalleryHoverPage = lazy(() => import("./pages/GalleryHoverPage"));
const BlogPage = lazy(() => import("./pages/BlogPage"));
const BlogDetailsPage = lazy(() => import("./pages/BlogDetailsPage"));
const AddBlogPage = lazy(() => import("./pages/AddBlogPage"));

const TestimonialsPage = lazy(() => import("./pages/TestimonialsPage"));
const FaqPage = lazy(() => import("./pages/FaqPage"));
const PricingPage = lazy(() => import("./pages/PricingPage"));
const TermsConditionPage = lazy(() => import("./pages/TermsConditionPage"));
const BlankPagePage = lazy(() => import("./pages/BlankPagePage"));
const ViewDetailsPage = lazy(() => import("./pages/ViewDetailsPage"));

const ComingSoonPage = lazy(() => import("./pages/ComingSoonPage"));
const AccessDeniedPage = lazy(() => import("./pages/AccessDeniedPage"));
const MaintenancePage = lazy(() => import("./pages/MaintenancePage"));
const SignInPage = lazy(() => import("./pages/SignInPage"));
const SignUpPage = lazy(() => import("./pages/SignUpPage"));
const ForgotPasswordPage = lazy(() => import("./pages/ForgotPasswordPage"));


const ErrorPage = lazy(() => import("./pages/ErrorPage"));

function App() {
  return (
    <>
      <RouteScrollToTop />
      <Routes>

        <Route path='add-user' element={<W C={AddUserPage} />} />
        <Route path='users-list' element={<W C={UsersListPage} />} />
        <Route path='users-grid' element={<W C={UsersGridPage} />} />
        <Route path='view-profile' element={<W C={ViewProfilePage} />} />
        <Route path='assign-role' element={<W C={AssignRolePage} />} />
        <Route path='role-access' element={<W C={RoleAccessPage} />} />


        <Route path='form' element={<W C={FormPage} />} />
        <Route path='form-layout' element={<W C={FormLayoutPage} />} />
        <Route path='form-validation' element={<W C={FormValidationPage} />} />
        <Route path='wizard' element={<W C={WizardPage} />} />
        <Route path='image-upload' element={<W C={ImageUploadPage} />} />

        <Route path='table-basic' element={<W C={TableBasicPage} />} />
        <Route path='table-data' element={<W C={TableDataPage} />} />

        <Route path='line-chart' element={<W C={LineChartPage} />} />
        <Route path='column-chart' element={<W C={ColumnChartPage} />} />
        <Route path='pie-chart' element={<W C={PieChartPage} />} />
        <Route path='widgets' element={<W C={WidgetsPage} />} />

        <Route path='email' element={<W C={EmailPage} />} />
        <Route path='starred' element={<W C={StarredPage} />} />
        <Route path='chat-empty' element={<W C={ChatEmptyPage} />} />
        <Route path='chat-message' element={<W C={ChatMessagePage} />} />
        <Route path='chat-profile' element={<W C={ChatProfilePage} />} />


        <Route path='text-generator' element={<W C={TextGeneratorPage} />} />
        <Route path='text-generator-new' element={<W C={TextGeneratorNewPage} />} />
        <Route path='code-generator' element={<W C={CodeGeneratorPage} />} />
        <Route path='code-generator-new' element={<W C={CodeGeneratorNewPage} />} />
        <Route path='image-generator' element={<W C={ImageGeneratorPage} />} />
        <Route path='voice-generator' element={<W C={VoiceGeneratorPage} />} />
        <Route path='video-generator' element={<W C={VideoGeneratorPage} />} />

        <Route path='wallet' element={<W C={WalletPage} />} />
        <Route path='marketplace' element={<W C={MarketplacePage} />} />
        <Route path='marketplace-details' element={<W C={MarketplaceDetailsPage} />} />
        <Route path='portfolio' element={<W C={PortfolioPage} />} />

        <Route path='gallery' element={<W C={GalleryPage} />} />
        <Route path='gallery-grid' element={<W C={GalleryGridPage} />} />
        <Route path='gallery-masonry' element={<W C={GalleryMasonryPage} />} />
        <Route path='gallery-hover' element={<W C={GalleryHoverPage} />} />
        <Route path='blog' element={<W C={BlogPage} />} />
        <Route path='blog-details' element={<W C={BlogDetailsPage} />} />
        <Route path='add-blog' element={<W C={AddBlogPage} />} />

        <Route path='testimonials' element={<W C={TestimonialsPage} />} />
        <Route path='faq' element={<W C={FaqPage} />} />
        <Route path='pricing' element={<W C={PricingPage} />} />
        <Route path='terms-condition' element={<W C={TermsConditionPage} />} />
        <Route path='blank-page' element={<W C={BlankPagePage} />} />
        <Route path='view-details' element={<W C={ViewDetailsPage} />} />

        <Route path='coming-soon' element={<W C={ComingSoonPage} />} />
        <Route path='access-denied' element={<W C={AccessDeniedPage} />} />
        <Route path='maintenance' element={<W C={MaintenancePage} />} />
        <Route path='sign-in' element={<W C={SignInPage} />} />
        <Route path='sign-up' element={<W C={SignUpPage} />} />
        <Route path='forgot-password' element={<W C={ForgotPasswordPage} />} />


        <Route path='*' element={<W C={ErrorPage} />} />
      </Routes>
    </>
  );
}

export default App;
