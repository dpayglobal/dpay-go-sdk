# Changelog

Wszystkie istotne zmiany w tym projekcie są dokumentowane w tym pliku.
Format oparty na [Keep a Changelog](https://keepachangelog.com/pl/1.1.0/),
wersjonowanie zgodne z [SemVer](https://semver.org/lang/pl/).

## [0.2.0] - wydanie razem z wdrożeniem API dpay

Wersja wymaga API dpay z tym samym wydaniem (wspólne API płatności cyklicznych, suma kontrolna capture
i anulowania kart). Zmiany łamiące zgodność są oznaczone jako **BREAKING**.

### Added

- `client.Recurring` (`RecurringService`): `Status()`, `Retry()` i `Cancel()` płatności cyklicznej
  (`/api/v1_0/payments/recurring/*`), modele `RecurringStatus`, `RecurringRegistrationInfo`, `RecurringRetryResult`
  i typy `RecurringState`, `RecurringRetryStatus`, `RecurringModel`, `RecurringMethod`.
- `RegisterPaymentRequest.RecurringRegistration` (`RecurringRegistration`) - rejestracja płatności cyklicznej
  (modele O, A i M, `TermsURL` wymagany) z kodem BLIK klienta; reguły modeli sprawdzane przed wysłaniem.
- `RegisterPaymentRequest.RecurringAlias` - obciążenie zapisanej płatności cyklicznej bez kodu BLIK;
  alias wchodzi do sumy kontrolnej. `UserAgent` i `UserIP` są przy obciążeniu opcjonalne.
- Webhooki: `VerifyWebhook()` i `WebhookVerifier` (`ConstructEvent()`, `Verify()`; Standard Webhooks, podpis `v1`,
  tolerancja czasu, kilka podpisów i sekretów w czasie rotacji), `WebhookEvent`, `WebhookEventType` z listami
  `MerchantEventTypes()`, `PaymentRegistrationEventTypes()`, `RefundEventTypes()` i `CaptureEventTypes()`.
- `client.Events` (`EventService`): historia zdarzeń z filtrami (`EventListParams`), `List()` i `Iterate()`
  po stronach (`EventPage`, `EventIterator`).
- `WebhookTarget` - własny adres zdarzeń w rejestracji płatności (`RegisterPaymentRequest.Webhook`), zwrocie
  (`WithRefundWebhook()`) i capture karty (`WithCaptureWebhook()`); `RegisterPaymentRequest.Reference`.
- `APIError.Reason`, `PaymentRejectedError.ErrorDescription`, `RegisteredPayment.RecurringAlias()`
  i `RegisteredPayment.RecurringMethods()`.
- `testdata/api_vectors.json` - wspólne wektory sum kontrolnych i podpisów webhooków wszystkich SDK dpay.

### Changed

- **BREAKING** `Cards.Capture()` i `Cards.Cancel()` wysyłają `service` i sumę
  `sha256(operacja|service|transaction_id|amount|hash)` - API odrzuca je bez sumy (401).
- **BREAKING** `Cards.Capture(ctx, transactionID, amount Money, opts ...CaptureOption)` - kwota jest wymagana
  (API odrzuca capture bez kwoty), zamiast `*Money`.
- **BREAKING** `ReturnURLs.IPN` jest opcjonalny: bez adresu `url_ipn` nie jest wysyłany, a IPN nie przychodzi
  (wynik przychodzi webhookiem).
- `APIError.ErrorCode` czyta kod błędu z pola `code` (np. `CHECKSUM_REQUIRED`, `WEBHOOK_URL_INVALID`), potem
  z `errorcode`.
- Suma kontrolna liczona po body (zwroty, szczegóły transakcji, banki, wypłaty) spłaszcza obiekty zagnieżdżone
  (np. `webhook`) w kolejności wysyłki.
- `RegisterPaymentRequest.UserIP` musi być adresem IPv4 albo IPv6 (jak `withClientContext()` w SDK PHP).

### Removed

- **BREAKING** `Blik.RecurringStatus()`, `BlikRecurringStatus`, `BlikRecurringInfo`, `BlikRecurringRegistration`,
  `BlikRecurringModel` i `RegisterPaymentRequest.RegisterBlikRecurringAlias` - API usunęło te endpointy i pole;
  użyj `client.Recurring` i `RegisterPaymentRequest.RecurringRegistration`.
- **BREAKING** `BlikAliasTypePayID` (aliasy OneClick są tylko `UID`), `TransactionTypeBlikRecurring`
  i `TransactionTypeBizumDirect` (API odrzuca je kodem 422).

### Deprecated

- `IPNTypeCapture`, `IPNEvent.IsCapture()` i `IPNEvent.CapturePaymentID()` - dpay nie wysyła już IPN typu
  `capture`; użyj zdarzenia `payment.captured`.

## [0.1.0] - 2026-07-22

Pierwsze wydanie.
