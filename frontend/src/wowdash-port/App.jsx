import { Route, Routes } from "react-router-dom";
import HomePageOne from "./pages/HomePageOne";
import HomePageTwo from "./pages/HomePageTwo";
import HomePageThree from "./pages/HomePageThree";
import HomePageFour from "./pages/HomePageFour";
import HomePageFive from "./pages/HomePageFive";
import HomePageSix from "./pages/HomePageSix";
import HomePageSeven from "./pages/HomePageSeven";
import EmailPage from "./pages/EmailPage";
import AddUserPage from "./pages/AddUserPage";
import AlertPage from "./pages/AlertPage";
import AssignRolePage from "./pages/AssignRolePage";
import AvatarPage from "./pages/AvatarPage";
import BadgesPage from "./pages/BadgesPage";
import ButtonPage from "./pages/ButtonPage";
import CalendarMainPage from "./pages/CalendarMainPage";
import CardPage from "./pages/CardPage";
import CarouselPage from "./pages/CarouselPage";
import ChatEmptyPage from "./pages/ChatEmptyPage";
import ChatMessagePage from "./pages/ChatMessagePage";
import ChatProfilePage from "./pages/ChatProfilePage";
import CodeGeneratorNewPage from "./pages/CodeGeneratorNewPage";
import CodeGeneratorPage from "./pages/CodeGeneratorPage";
import ColorsPage from "./pages/ColorsPage";
import ColumnChartPage from "./pages/ColumnChartPage";
import CompanyPage from "./pages/CompanyPage";
import CurrenciesPage from "./pages/CurrenciesPage";
import DropdownPage from "./pages/DropdownPage";
import ErrorPage from "./pages/ErrorPage";
import FaqPage from "./pages/FaqPage";
import ForgotPasswordPage from "./pages/ForgotPasswordPage";
import FormLayoutPage from "./pages/FormLayoutPage";
import FormValidationPage from "./pages/FormValidationPage";
import FormPage from "./pages/FormPage";
import GalleryPage from "./pages/GalleryPage";
import ImageGeneratorPage from "./pages/ImageGeneratorPage";
import ImageUploadPage from "./pages/ImageUploadPage";
import InvoiceAddPage from "./pages/InvoiceAddPage";
import InvoiceEditPage from "./pages/InvoiceEditPage";
import InvoiceListPage from "./pages/InvoiceListPage";
import InvoicePreviewPage from "./pages/InvoicePreviewPage";
import KanbanPage from "./pages/KanbanPage";
import LanguagePage from "./pages/LanguagePage";
import LineChartPage from "./pages/LineChartPage";
import ListPage from "./pages/ListPage";
import MarketplaceDetailsPage from "./pages/MarketplaceDetailsPage";
import MarketplacePage from "./pages/MarketplacePage";
import NotificationAlertPage from "./pages/NotificationAlertPage";
import NotificationPage from "./pages/NotificationPage";
import PaginationPage from "./pages/PaginationPage";
import PaymentGatewayPage from "./pages/PaymentGatewayPage";
import PieChartPage from "./pages/PieChartPage";
import PortfolioPage from "./pages/PortfolioPage";
import PricingPage from "./pages/PricingPage";
import ProgressPage from "./pages/ProgressPage";
import RadioPage from "./pages/RadioPage";
import RoleAccessPage from "./pages/RoleAccessPage";
import SignInPage from "./pages/SignInPage";
import SignUpPage from "./pages/SignUpPage";
import StarRatingPage from "./pages/StarRatingPage";
import StarredPage from "./pages/StarredPage";
import SwitchPage from "./pages/SwitchPage";
import TableBasicPage from "./pages/TableBasicPage";
import TableDataPage from "./pages/TableDataPage";
import TabsPage from "./pages/TabsPage";
import TagsPage from "./pages/TagsPage";
import TermsConditionPage from "./pages/TermsConditionPage";
import TextGeneratorPage from "./pages/TextGeneratorPage";
import ThemePage from "./pages/ThemePage";
import TooltipPage from "./pages/TooltipPage";
import TypographyPage from "./pages/TypographyPage";
import UsersGridPage from "./pages/UsersGridPage";
import UsersListPage from "./pages/UsersListPage";
import ViewDetailsPage from "./pages/ViewDetailsPage";
import VideoGeneratorPage from "./pages/VideoGeneratorPage";
import VideosPage from "./pages/VideosPage";
import ViewProfilePage from "./pages/ViewProfilePage";
import VoiceGeneratorPage from "./pages/VoiceGeneratorPage";
import WalletPage from "./pages/WalletPage";
import WidgetsPage from "./pages/WidgetsPage";
import WizardPage from "./pages/WizardPage";
import RouteScrollToTop from "./helper/RouteScrollToTop";
import TextGeneratorNewPage from "./pages/TextGeneratorNewPage";
import HomePageEight from "./pages/HomePageEight";
import HomePageNine from "./pages/HomePageNine";
import HomePageTen from "./pages/HomePageTen";
import HomePageEleven from "./pages/HomePageEleven";
import GalleryGridPage from "./pages/GalleryGridPage";
import GalleryMasonryPage from "./pages/GalleryMasonryPage";
import GalleryHoverPage from "./pages/GalleryHoverPage";
import BlogPage from "./pages/BlogPage";
import BlogDetailsPage from "./pages/BlogDetailsPage";
import AddBlogPage from "./pages/AddBlogPage";
import TestimonialsPage from "./pages/TestimonialsPage";
import ComingSoonPage from "./pages/ComingSoonPage";
import AccessDeniedPage from "./pages/AccessDeniedPage";
import MaintenancePage from "./pages/MaintenancePage";
import BlankPagePage from "./pages/BlankPagePage";

function App() {
  return (
    <>
      <RouteScrollToTop />
      <Routes>
        <Route index element={<HomePageOne />} />
        <Route path='index-2' element={<HomePageTwo />} />
        <Route path='index-3' element={<HomePageThree />} />
        <Route path='index-4' element={<HomePageFour />} />
        <Route path='index-5' element={<HomePageFive />} />
        <Route path='index-6' element={<HomePageSix />} />
        <Route path='index-7' element={<HomePageSeven />} />
        <Route path='index-8' element={<HomePageEight />} />
        <Route path='index-9' element={<HomePageNine />} />
        <Route path='index-10' element={<HomePageTen />} />
        <Route path='index-11' element={<HomePageEleven />} />

        {/* SL */}
        <Route path='add-user' element={<AddUserPage />} />
        <Route path='alert' element={<AlertPage />} />
        <Route path='assign-role' element={<AssignRolePage />} />
        <Route path='avatar' element={<AvatarPage />} />
        <Route path='badges' element={<BadgesPage />} />
        <Route path='button' element={<ButtonPage />} />
        <Route path='calendar-main' element={<CalendarMainPage />} />
        <Route path='calendar' element={<CalendarMainPage />} />
        <Route path='card' element={<CardPage />} />
        <Route path='carousel' element={<CarouselPage />} />
        <Route path='chat-empty' element={<ChatEmptyPage />} />
        <Route path='chat-message' element={<ChatMessagePage />} />
        <Route path='chat-profile' element={<ChatProfilePage />} />
        <Route path='code-generator' element={<CodeGeneratorPage />} />
        <Route
          path='code-generator-new'
          element={<CodeGeneratorNewPage />}
        />
        <Route path='colors' element={<ColorsPage />} />
        <Route path='column-chart' element={<ColumnChartPage />} />
        <Route path='company' element={<CompanyPage />} />
        <Route path='currencies' element={<CurrenciesPage />} />
        <Route path='dropdown' element={<DropdownPage />} />
        <Route path='email' element={<EmailPage />} />
        <Route path='faq' element={<FaqPage />} />
        <Route path='forgot-password' element={<ForgotPasswordPage />} />
        <Route path='form-layout' element={<FormLayoutPage />} />
        <Route path='form-validation' element={<FormValidationPage />} />
        <Route path='form' element={<FormPage />} />

        <Route path='gallery' element={<GalleryPage />} />
        <Route path='gallery-grid' element={<GalleryGridPage />} />
        <Route path='gallery-masonry' element={<GalleryMasonryPage />} />
        <Route path='gallery-hover' element={<GalleryHoverPage />} />

        <Route path='blog' element={<BlogPage />} />
        <Route path='blog-details' element={<BlogDetailsPage />} />
        <Route path='add-blog' element={<AddBlogPage />} />

        <Route path='testimonials' element={<TestimonialsPage />} />
        <Route path='coming-soon' element={<ComingSoonPage />} />
        <Route path='access-denied' element={<AccessDeniedPage />} />
        <Route path='maintenance' element={<MaintenancePage />} />
        <Route path='blank-page' element={<BlankPagePage />} />

        <Route path='image-generator' element={<ImageGeneratorPage />} />
        <Route path='image-upload' element={<ImageUploadPage />} />
        <Route path='invoice-add' element={<InvoiceAddPage />} />
        <Route path='invoice-edit' element={<InvoiceEditPage />} />
        <Route path='invoice-list' element={<InvoiceListPage />} />
        <Route path='invoice-preview' element={<InvoicePreviewPage />} />
        <Route path='kanban' element={<KanbanPage />} />
        <Route path='language' element={<LanguagePage />} />
        <Route path='line-chart' element={<LineChartPage />} />
        <Route path='list' element={<ListPage />} />
        <Route
          path='marketplace-details'
          element={<MarketplaceDetailsPage />}
        />
        <Route path='marketplace' element={<MarketplacePage />} />
        <Route
          path='notification-alert'
          element={<NotificationAlertPage />}
        />
        <Route path='notification' element={<NotificationPage />} />
        <Route path='pagination' element={<PaginationPage />} />
        <Route path='payment-gateway' element={<PaymentGatewayPage />} />
        <Route path='pie-chart' element={<PieChartPage />} />
        <Route path='portfolio' element={<PortfolioPage />} />
        <Route path='pricing' element={<PricingPage />} />
        <Route path='progress' element={<ProgressPage />} />
        <Route path='radio' element={<RadioPage />} />
        <Route path='role-access' element={<RoleAccessPage />} />
        <Route path='sign-in' element={<SignInPage />} />
        <Route path='sign-up' element={<SignUpPage />} />
        <Route path='star-rating' element={<StarRatingPage />} />
        <Route path='starred' element={<StarredPage />} />
        <Route path='switch' element={<SwitchPage />} />
        <Route path='table-basic' element={<TableBasicPage />} />
        <Route path='table-data' element={<TableDataPage />} />
        <Route path='tabs' element={<TabsPage />} />
        <Route path='tags' element={<TagsPage />} />
        <Route path='terms-condition' element={<TermsConditionPage />} />
        <Route
          path='text-generator-new'
          element={<TextGeneratorNewPage />}
        />
        <Route path='text-generator' element={<TextGeneratorPage />} />
        <Route path='theme' element={<ThemePage />} />
        <Route path='tooltip' element={<TooltipPage />} />
        <Route path='typography' element={<TypographyPage />} />
        <Route path='users-grid' element={<UsersGridPage />} />
        <Route path='users-list' element={<UsersListPage />} />
        <Route path='view-details' element={<ViewDetailsPage />} />
        <Route path='video-generator' element={<VideoGeneratorPage />} />
        <Route path='videos' element={<VideosPage />} />
        <Route path='view-profile' element={<ViewProfilePage />} />
        <Route path='voice-generator' element={<VoiceGeneratorPage />} />
        <Route path='wallet' element={<WalletPage />} />
        <Route path='widgets' element={<WidgetsPage />} />
        <Route path='wizard' element={<WizardPage />} />

        <Route path='*' element={<ErrorPage />} />
      </Routes>
    </>
  );
}

export default App;
