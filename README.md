# dpay Go SDK

Oficjalna biblioteka Go do integracji z API płatności [dpay.pl](https://dpay.pl).

## Wymagania

- Go 1.22 lub nowszy
- Zero zależności runtime - wyłącznie biblioteka standardowa

## Instalacja

```bash
go get github.com/dpayglobal/dpay-go-sdk
```

## Szybki start

```go
import dpay "github.com/dpayglobal/dpay-go-sdk"

client, err := dpay.New("nazwa_serwisu", "twoj_secret_hash")
if err != nil {
    return err
}

payment, err := client.Payments.Register(ctx, &dpay.RegisterPaymentRequest{
    Amount:          dpay.PLN(1050),
    TransactionType: dpay.TransactionTypeTransfers,
    URLs: dpay.ReturnURLs{
        Success: "https://twojsklep.pl/sukces",
        Fail:    "https://twojsklep.pl/blad",
        IPN:     "https://twojsklep.pl/ipn",
    },
    Description: dpay.String("Zamówienie #1234"),
    Custom:      dpay.String("order-1234"),
})
if err != nil {
    return err
}

if url := payment.RedirectURL(); url != "" {
    http.Redirect(w, r, url, http.StatusSeeOther)
}
```

Pola opcjonalne są wskaźnikami. `nil` pomija pole, a `dpay.Bool(false)` wysyła
`false` - to rozróżnienie ma znaczenie dla API. Do budowania wskaźników służą
`dpay.String`, `dpay.Bool`, `dpay.Int` i `dpay.Int64`.

## Płatności cykliczne

Rejestracja idzie razem z płatnością kodem BLIK klienta (kwota `0` - sama zgoda, więcej - opłata inicjalna).
Kolejne obciążenia wysyła Twój serwer, bez kodu.

```go
urls := dpay.ReturnURLs{Success: "https://twojsklep.pl/sukces", Fail: "https://twojsklep.pl/blad"}

registration, err := client.Payments.Register(ctx, &dpay.RegisterPaymentRequest{
    Amount:          dpay.PLN(0),
    TransactionType: dpay.TransactionTypeTransfers,
    URLs:            urls,
    BlikCode:        dpay.String(kodBlik),
    UserAgent:       dpay.String(r.UserAgent()),
    UserIP:          dpay.String(clientIP),
    RecurringRegistration: &dpay.RecurringRegistration{
        Label:    "Abonament Premium",
        Model:    dpay.RecurringModelOnDemand,
        TermsURL: "https://twojsklep.pl/regulamin",
        Alias:    dpay.String("SUB-1234"),
    },
})

charge, err := client.Payments.Register(ctx, &dpay.RegisterPaymentRequest{
    Amount:          dpay.PLN(4999),
    TransactionType: dpay.TransactionTypeTransfers,
    URLs:            urls,
    RecurringAlias:  dpay.String("SUB-1234"),
    Description:     dpay.String("Abonament Premium 10/2026"),
})

status, err := client.Recurring.Status(ctx, "SUB-1234")   // ACTIVE, INACTIVE, UNREGISTERED, EXPIRED, DECLINED
retry, err := client.Recurring.Retry(ctx, charge.TransactionID()) // po odmowie, np. INSUFFICIENT_FUNDS
state, err := client.Recurring.Cancel(ctx, "SUB-1234", dpay.WithCancelReason("Rezygnacja klienta"))
```

Model A (`RecurringModelAutomatic`) wymaga `Frequency`, `LimitAmt`, `TotLimitAmt`, `ExpirationDate`
i `InitDate`, model O (`RecurringModelOnDemand`) ich zabrania, w modelu M (`RecurringModelManual`)
są opcjonalne - SDK sprawdza to przed wysłaniem. Kwoty limitów podajesz w groszach.

Obciążenie wiąże alias z sumą kontrolną, a anulowanie ma własną sumę - SDK liczy obie.
Ponowienie, którego API nie dopuszcza, kończy się błędem `ErrInvalidRequest` z powodem w
`apiErr.FieldErrors["retry"]` (np. `DECLINE_NOT_RETRYABLE`). Limity API: `Status` do 60,
`Retry` i `Cancel` do 30 zapytań na minutę (licznik wspólny z resztą API płatności z tego
adresu IP) - nie odpytuj statusu w pętli, wynik przychodzi webhookiem.

## Webhooki

Zdarzenia (`payment.succeeded`, `refund.failed`, `recurring_payment.canceled` i inne) są podpisane.
Weryfikuj je na surowym body, przed parsowaniem JSON:

```go
func webhookHandler(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "cannot read body", http.StatusBadRequest)
        return
    }

    // sekret endpointu z panelu; w czasie rotacji podaj oba sekrety
    event, err := dpay.VerifyWebhook(body, r.Header, "whsec_...")
    if err != nil {
        http.Error(w, "invalid signature", http.StatusBadRequest)
        return
    }

    if event.Type() == dpay.WebhookEventTypePaymentSucceeded {
        payment := event.Object() // kwoty w groszach
        _ = payment
    }
    w.WriteHeader(http.StatusOK)
}
```

`VerifyWebhook` sprawdza podpis `v1` i znacznik czasu (tolerancja 5 minut). Inną tolerancję
albo własny zegar ustawisz w `dpay.WebhookVerifier{Secrets: ..., Tolerance: ..., Now: ...}`.
Niepoprawny podpis to `ErrSignature`, a sekret, który nie jest base64, to `ErrInvalidArgument`.

Deduplikuj zdarzenia po `event.ID()`. Historię zdarzeń (np. po awarii endpointu) pobierzesz przez
`client.Events.Iterate`:

```go
events := client.Events.Iterate(ctx, &dpay.EventListParams{
    Types: []dpay.WebhookEventType{dpay.WebhookEventTypePaymentSucceeded},
})
for events.Next() {
    handle(events.Event())
}
if err := events.Err(); err != nil {
    // ...
}
```

Własny adres zdarzeń jednej płatności (podpisywany sekretem webhooków serwisu):
`RegisterPaymentRequest.Webhook = &dpay.WebhookTarget{URL: "https://twojsklep.pl/webhooks"}`.
Pole `Reference` (do 64 znaków) wraca w zdarzeniach jako `references.merchant`.

## Obsługa IPN

IPN przychodzi tylko wtedy, gdy podasz adres IPN w `ReturnURLs.IPN`.

dpay.pl uznaje IPN za dostarczony wyłącznie, gdy body odpowiedzi to dokładnie `OK`.
Kod HTTP nie jest sprawdzany. Zawsze weryfikuj kwotę z własnym zamówieniem.

```go
func ipnHandler(w http.ResponseWriter, r *http.Request) {
    body, err := io.ReadAll(r.Body)
    if err != nil {
        http.Error(w, "cannot read body", http.StatusBadRequest)
        return
    }

    event, err := dpay.VerifyIPN(body, "twoj_secret_hash")
    if err != nil {
        http.Error(w, "invalid signature", http.StatusBadRequest)
        return
    }

    if event.IsTransfer() {
        markOrderAsPaid(event.ID(), event.Amount())
    }

    fmt.Fprint(w, dpay.IPNAck)
}
```

`event.Amount()` to surowy string dziesiętny - payload IPN nie niesie waluty,
więc porównaj go z kwotą własnego zamówienia.

## Zwroty

```go
client.Refunds.Create(ctx, "identyfikator-transakcji")
client.Refunds.Create(ctx, "identyfikator-transakcji",
    dpay.WithRefundAmount(dpay.PLN(500)),
    dpay.WithRefundReason("reklamacja"))

// Odpowiedź oznacza przyjęcie zwrotu - wynik przychodzi zdarzeniem refund.succeeded / refund.failed
client.Refunds.Create(ctx, "identyfikator-transakcji",
    dpay.WithRefundAmount(dpay.PLN(500)),
    dpay.WithRefundWebhook(dpay.WebhookTarget{
        URL: "https://twojsklep.pl/webhooks/zwroty",
        Events: []dpay.WebhookEventType{
            dpay.WebhookEventTypeRefundSucceeded,
            dpay.WebhookEventTypeRefundFailed,
        },
    }))

availability, err := client.Refunds.CheckAvailability(ctx, "identyfikator-transakcji")
if err == nil && availability.IsAvailable() {
    // ...
}
```

## Szczegóły transakcji i banki

```go
transaction, err := client.Payments.Details(ctx, "identyfikator-transakcji")
transaction.IsPaid()
transaction.AvailableRefundAmount().String()
transaction.Refunds()

banks, err := client.Banks.ForService(ctx)
```

## Karty S2S

```go
publicKey, err := client.Cards.PublicKey(ctx)
encrypted, err := dpay.EncryptCard(
    dpay.CardData{PAN: "4111111111111111", CVV: "123", Expiry: "12/28"},
    transactionID,
    publicKey,
)

result, err := client.Cards.PayOTP(ctx, transactionID, &dpay.CardPaymentRequest{
    DeviceInfo:        deviceInfo,
    EncryptedCardData: dpay.String(encrypted),
})

switch {
case result.RequiresThreeDSForm():
    fmt.Fprint(w, result.ThreeDSFormHTML())
case result.RequiresRedirect():
    http.Redirect(w, r, result.RedirectURL(), http.StatusSeeOther)
case result.HasDCCOffer():
    offer := result.DCCOffer()
    _ = offer.DeclarationText()
}
```

Klucz publiczny jest rotowany - pobieraj go przed każdą próbą płatności,
nie buforuj.

Capture wymaga kwoty (częściowe pobrania do wysokości autoryzacji), anulowanie
bez kwoty (`nil`) zwalnia całą niepobraną resztę. SDK dodaje do obu `service`
i sumę kontrolną operacji:

```go
client.Cards.Capture(ctx, transactionID, dpay.PLN(2999))
client.Cards.Capture(ctx, transactionID, dpay.PLN(2999),
    dpay.WithCaptureWebhook(dpay.WebhookTarget{URL: "https://twojsklep.pl/webhooks/capture"}))
client.Cards.Cancel(ctx, transactionID, nil)
```

Wynik capture przychodzi zdarzeniem `payment.captured` - IPN typu `capture` nie jest już wysyłany.

## Obsługa błędów

Rodzaj błędu rozpoznaje się przez `errors.Is`, a szczegóły przez `errors.As`.

```go
payment, err := client.Payments.Register(ctx, request)

switch {
case errors.Is(err, dpay.ErrInvalidRequest):
    var apiErr *dpay.APIError
    errors.As(err, &apiErr)
    log.Print(apiErr.FieldErrors, apiErr.ErrorCode, apiErr.Reason) // np. WEBHOOK_URL_INVALID, https_required

case errors.Is(err, dpay.ErrPaymentRejected):
    var rejected *dpay.PaymentRejectedError
    errors.As(err, &rejected)
    log.Print(rejected.TransactionID, rejected.ErrorCode)

case errors.Is(err, dpay.ErrTransport):
    // błąd sieci - status płatności nieznany, użyj Payments.Details
}
```

| Sentinel | Kiedy |
|---|---|
| `ErrAuthentication` | 401 - niepoprawny albo brakujący checksum (`CHECKSUM_REQUIRED`, `INVALID_CHECKSUM`) |
| `ErrInvalidRequest` | 400, 422 |
| `ErrAccessDenied` | 403 |
| `ErrNotFound` | 404 |
| `ErrRateLimit` | 429, `*RateLimitError` niesie `RetryAfter` |
| `ErrServer` | 5xx |
| `ErrPaymentRejected` | rejestracja odrzucona przy HTTP 200 |
| `ErrCardPayment` | płatność kartą odrzucona przy HTTP 200 |
| `ErrSignature` | niepoprawny podpis IPN albo webhooka |
| `ErrTransport` | awaria sieci |
| `ErrInvalidArgument` | niepoprawny argument lub pole żądania |
| `ErrAPI` | pasuje do każdego błędu zwróconego przez API |

## Konfiguracja

| Opcja | Typ | Opis |
|---|---|---|
| `dpay.New(service, secretHash)` | `string`, `string` | Nazwa Punktu Płatności z panel.dpay.pl i Secret Hash (wymagane) |
| `WithTimeout` | `time.Duration` | Timeout domyślnego klienta HTTP (domyślnie 30 s) |
| `WithHTTPClient` | `HTTPDoer` | Własny transport - proxy, retry, instrumentacja, testy |
| `WithBaseURLs` | `BaseURLs` | Nadpisanie hostów API |

`HTTPDoer` to jednometodowy interfejs `Do(*http.Request) (*http.Response, error)`,
który `*http.Client` spełnia bez adaptera. Retry i timeouty per żądanie
konfiguruje się własnym `http.Client` lub `RoundTripper`. Anulowanie przez
`context.Context` działa w każdej metodzie.

Domyślny klient **nie podąża za przekierowaniami** - odpowiedź 302 jest
zwracana jako wynik, a nie zamieniana na stronę, pod którą prowadzi. Klient
płatniczy nie powinien ponawiać żądania pod adres wskazany przez odpowiedź,
a dodatkowo tak zachowuje się SDK PHP, więc statusy błędów są w obu takie same.
Jeśli podajesz własny `*http.Client`, ustaw to samo:

```go
client, err := dpay.New("nazwa_serwisu", "twoj_secret_hash",
    dpay.WithHTTPClient(&http.Client{
        Timeout: 30 * time.Second,
        CheckRedirect: func(*http.Request, []*http.Request) error {
            return http.ErrUseLastResponse
        },
    }))
```

## Testowanie integracji

SDK nie dostarcza własnego mocka - wystarczy `httptest` z biblioteki standardowej:

```go
server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
    w.Write([]byte(`{"transactionId":"tx-1","msg":"https://secure.dpay.pl/pay/1"}`))
}))
defer server.Close()

client, _ := dpay.New("test", "test", dpay.WithBaseURLs(dpay.BaseURLs{
    APIPayments: server.URL,
    Panel:       server.URL,
}))
```

Jeśli wolisz sprawdzać wysłane żądania bez podnoszenia serwera, zaimplementuj
`dpay.HTTPDoer` - to jednometodowy interfejs, który spełnia też `*http.Client`.

## Zgodność z SDK PHP

To SDK jest portem `dpayglobal/dpay-php-sdk` i utrzymuje parytet na poziomie
wire-protocol: te same ścieżki, ta sama kolejność pól w body, te same checksumy
i podpisy IPN. Parytet jest mierzony, nie deklarowany - testy porównują żądania
bajt w bajt z wektorami wygenerowanymi z żywego SDK PHP, a sumy kontrolne
i podpisy webhooków ze wspólnymi wektorami wszystkich SDK dpay
(`testdata/api_vectors.json`).

## Licencja

Apache-2.0
