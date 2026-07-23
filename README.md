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

## Obsługa IPN

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

    if event.IsTransfer() || event.IsCapture() {
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

Capture pełnej kwoty wykonuje się przekazując `nil`:

```go
client.Cards.Capture(ctx, transactionID, nil)
client.Cards.Capture(ctx, transactionID, &amount)
```

## Obsługa błędów

Rodzaj błędu rozpoznaje się przez `errors.Is`, a szczegóły przez `errors.As`.

```go
payment, err := client.Payments.Register(ctx, request)

switch {
case errors.Is(err, dpay.ErrInvalidRequest):
    var apiErr *dpay.APIError
    errors.As(err, &apiErr)
    log.Print(apiErr.FieldErrors)

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
| `ErrAuthentication` | 401 - niepoprawny checksum |
| `ErrInvalidRequest` | 400, 422 |
| `ErrAccessDenied` | 403 |
| `ErrNotFound` | 404 |
| `ErrRateLimit` | 429, `*RateLimitError` niesie `RetryAfter` |
| `ErrServer` | 5xx |
| `ErrPaymentRejected` | rejestracja odrzucona przy HTTP 200 |
| `ErrCardPayment` | płatność kartą odrzucona przy HTTP 200 |
| `ErrSignature` | niepoprawny podpis IPN |
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
bajt w bajt z wektorami wygenerowanymi z żywego SDK PHP.

## Licencja

Apache-2.0
