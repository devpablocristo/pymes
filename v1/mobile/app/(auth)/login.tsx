import { useSignIn } from "@clerk/expo/legacy";
import { useState } from "react";
import { Alert, KeyboardAvoidingView, StyleSheet } from "react-native";

import { DSButton } from "@/components/ui/button";
import { DSLayout } from "@/components/ui/layout";
import { DSVStack } from "@/components/ui/stack";
import { DSText } from "@/components/ui/text";
import { DSTextInput } from "@/components/ui/text-input";
import { Spacing } from "@/constants/theme";
import { t } from "@/constants/translations";

export default function LoginScreen() {
  const { signIn, setActive, isLoaded } = useSignIn();

  // TODO: remove mock credentials before release
  const [email, setEmail] = useState("santiagoxxviii+clerk_test@gmail.com");
  const [password, setPassword] = useState("12345678");
  const [loading, setLoading] = useState(false);

  async function handleLogin() {
    if (!isLoaded) return;
    if (!email.trim() || !password.trim()) {
      Alert.alert("Error", t.login.errors.emptyFields);
      return;
    }

    setLoading(true);
    try {
      const result = await signIn.create({
        identifier: email.trim(),
        password,
      });

      if (result.status === "complete") {
        await setActive({ session: result.createdSessionId });
      } else {
        Alert.alert("Error", t.login.errors.loginFailed);
      }
    } catch (err: any) {
      const message =
        err?.errors?.[0]?.longMessage ?? t.login.errors.loginFailed;
      Alert.alert("Error", message);
    } finally {
      setLoading(false);
    }
  }

  return (
    <KeyboardAvoidingView style={styles.keyboardView} behavior="padding">
      <DSLayout scrollable contentStyle={styles.content}>
        <DSVStack style={styles.header} gap="md">
          <DSText variant="title">{t.login.appName}</DSText>
          <DSText variant="paragraph" color="muted">
            {t.login.subtitle}
          </DSText>
        </DSVStack>

        <DSVStack gap="md">
          <DSText variant="label">{t.login.email}</DSText>
          <DSTextInput
            placeholder={t.login.emailPlaceholder}
            value={email}
            onChangeText={setEmail}
            autoCapitalize="none"
            keyboardType="email-address"
            returnKeyType="next"
            editable={!loading}
          />

          <DSText variant="label">{t.login.password}</DSText>
          <DSTextInput
            placeholder={t.login.passwordPlaceholder}
            value={password}
            onChangeText={setPassword}
            secureTextEntry
            returnKeyType="done"
            onSubmitEditing={handleLogin}
            editable={!loading}
          />

          <DSButton
            title={t.login.submit}
            onPress={handleLogin}
            loading={loading}
            style={styles.button}
          />
        </DSVStack>
      </DSLayout>
    </KeyboardAvoidingView>
  );
}

const styles = StyleSheet.create({
  keyboardView: {
    flex: 1,
  },
  content: {
    justifyContent: "center",
  },
  header: {
    marginBottom: Spacing.xl,
  },
  button: {
    marginTop: Spacing.lg,
  },
});
