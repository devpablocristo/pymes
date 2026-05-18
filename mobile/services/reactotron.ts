import Reactotron from 'reactotron-react-native';

declare global {
  interface Console {
    tron: typeof Reactotron;
  }
}

if (__DEV__) {
  Reactotron.configure({ name: 'mobile' })
    .useReactNative({ networking: true })
    .connect();

  console.tron = Reactotron;
}
