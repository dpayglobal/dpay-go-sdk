# Changelog

Wszystkie istotne zmiany w tym projekcie są dokumentowane w tym pliku.
Format oparty na [Keep a Changelog](https://keepachangelog.com/pl/1.1.0/),
wersjonowanie zgodne z [SemVer](https://semver.org/lang/pl/).

## [0.3.0] - 2026-11-19

Wersja dogania dokumentację API dpay (OpenAPI 1.6.0) i działa z obecnym API - nie wymaga nowego wydania API.

### Breaking

- **BREAKING** `Refunds.CheckAvailability()` zwraca `RefundAvailability` (niedostępny) zamiast błędu dla każdej
  odpowiedzi 4xx z kluczem `refund`, w tym 404 (transakcja nie istnieje, wcześniej `ErrNotFound`) i 423 (zwrot
  przez BOK dpay). Przyczynę rozróżnia `HTTPStatus()`. Nadal są błędami: 401 "Unauthorized request"
  (`ErrAuthentication`), 422, 429 i 5xx.
- **BREAKING** Waluta kwoty trafia do `currency_code`: `Money` w walucie innej niż PLN wysyła `currency_code`
  bez `CurrencyCode` (wcześniej np. `dpay.NewMoney(49900, dpay.CurrencyCZK)` rejestrowało 499.00 w walucie
  domyślnej serwisu). `CurrencyCode` (także razem z `PhoneNumber`) różny od waluty kwoty kończy się błędem
  `ErrInvalidArgument` - dla MB WAY użyj `dpay.NewMoney(grosze, dpay.CurrencyEUR)` zamiast `dpay.PLN()`.
  Kwota w PLN bez jawnego kodu wysyła body jak dotąd (bez `currency_code`).
- **BREAKING** `VerifyIPN()` przyjmuje IPN `alias_update` (wcześniej zwracał `ErrSignature` "Invalid IPN
  payload"). Przychodzi on na adres IPN, gdy rejestracja nie miała `AliasIPNURL` - obsługa, która oznacza
  zamówienie jako opłacone bez sprawdzenia `IsTransfer()`, potraktowałaby zmianę aliasu (np. `ALIAS_DECLINED`)
  jak wpłatę. Dla `alias_update` `Amount()`, `Email()` i `Custom()` są puste, a `Attempt()` zwraca 0 (te pola
  nie są podpisane).

### Added

- IPN `alias_update` (BLIK OneClick i płatności cykliczne): `VerifyIPN()` weryfikuje podpis
  `sha256(id . hash . type . version)`, `IPNTypeAliasUpdate`, `IPNEvent.IsAliasUpdate()` i `AliasUpdate()`
  (`AliasUpdate` z typami `AliasChangeType`, `AliasType` i `AliasStatus`).
- `CardError` (`Code`, `DeclineCode`, `DeclineMessage`) oraz pola `APIError.CardError` i
  `APIError.TransactionID`: z obiektu `error` odmowy karty (`*CardPaymentError`) i z pól `cardError`
  i `transactionId` błędu Payments API (np. odrzucone pobranie na zapisanej karcie).
- `client.Payouts.List()` i `Iterate()` (`POST /api/v1/pbl/withdraws`, `PayoutListParams`: `DateFrom`,
  `DateTo`, `State`, `Type`, strony do 200; `PayoutIterator`) oraz `DetailsFull()`
  (`POST /api/v1/pbl/withdraws/details/full`: nagłówek, sumy całego rozliczenia i pozycje z `payment_id`,
  `custom`, `ipksef`, strony do 500, `PayoutPageParams`); modele `PayoutPage`, `PayoutListItem`,
  `PayoutSettlement`, `SettlementPosition`, `PageMeta` i typy `PayoutState`, `PayoutType`.
- `PayoutDetails.Currency()` (z `currency_code`, PLN gdy go brak), `ExportDate()` i `IsPaysafecard()`.
- `WithRefundMessageID()` dla `Refunds.Create()` - nagłówek `DigitalPayments-PSP-Message-ID`
  (`RefundMessageIDHeader`; tytuł przelewu `dpay.pl zwrot {messageID}`, poza sumą kontrolną).
- `RegisteredPayment.Multibanco()` (`MultibancoReference`: encja, referencja, kwota, ważność)
  i `PaidTransaction()` (`RegisteredTransaction` z odpowiedzi "Transaction paid", statusy
  `RegisteredTransactionStatus` pending, paid, captured, canceled).
- Typowane obiekty webhooków: `WebhookEvent.Payment()`, `Refund()`, `RecurringPayment()`, `Payout()`
  (`PaymentObject`, `RefundObject`, `RecurringPaymentObject`, `PayoutObject`), `Failure`, `DeclineCode`
  (22 kody odmów dpay, `DeclineCodes()`) i `FailureCategory` (`FailureCategories()`).
- `RecurringRetryResult.IsSuccess()` i `RetryRefusal` (kody `errors.retry` odmowy ponowienia, `RetryRefusals()`).
- `Transaction.Rate()`, `MinimalFee()`, `PermanentFee()` i `URLs()` (`TransactionURLs`).
- `ApplePayRequest.Email` i `CardPaymentResult.ApplePayMerchantSession()` (sesja ApplePayMerchantSession
  z `redirectText` odpowiedzi APPLE_PAY_INIT).
- `RateLimitError.Reset` (nagłówek `X-RateLimit-Reset`), `CurrencyUSD` i `CurrencyRON`, `IsValidPolishIBAN()`.
- `testdata/api_vectors.json`: wektory IPN transfer, dcb i alias_update oraz sum list i szczegółów wypłat;
  golden z SDK PHP 0.3.0 (39 wywołań i modele odpowiedzi 0.3.0).

### Changed

- Walidacja przed wysłaniem tego, co API i tak odrzuca (422 lub 400):
  - etykieta `RegisterBlikAlias` (`BlikAliasRegistration`) i `RecurringRegistration` do 35 znaków (limit BLIK,
    wcześniej 50);
  - `TransactionTypeMBWayDirect` wymaga kanału (`Channel: dpay.String("373")`) i `DeviceInfo`
    z `BrowserJavaEnabled`;
  - kanał bezpośredni wymaga emaila, imienia i nazwiska płatnika (`Payer`) oraz `AcceptTos`; kanały `tester`,
    `paypal`, `installment` i `paysafecard` są odrzucane; pusty kanał oznacza brak kanału, jak dotąd;
  - `PhoneNumber` 9-15 znaków, `AliasIPNURL` do 500 znaków;
  - `Efaktura` nie łączy się z `BlikCode`, `BlikAlias`, `RegisterBlikAlias`, płatnościami cyklicznymi
    ani aliasami kart;
  - `PayoutPosition`: IBAN `PL` + 26 cyfr z poprawną sumą kontrolną mod 97 (spacje i małe litery dozwolone)
    i kwota powyżej 0.
- `APIError.TransactionID` jest ustawiane także w `*PaymentRejectedError` (pole
  `PaymentRejectedError.TransactionID` zostaje, z tą samą wartością).
- Dodatkowe nagłówki żądania (dziś `DigitalPayments-PSP-Message-ID`) idą z nazwą dokładnie taką, jak w SDK PHP;
  nazwa spoza `[A-Za-z0-9-]` albo wartość ze znakiem sterującym kończy się błędem `ErrInvalidArgument`.

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
