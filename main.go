package main

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	"golang.org/x/oauth2"

	"github.com/nerdwarelabs/spapi"
)

var marketplace string

func main() {
	// get viper config
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")

	err := viper.ReadInConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config")
	}

	cmd := cobra.Command{
		Use: "nwl",
		Run: run,
	}

	cmd.PersistentFlags().StringVarP(&marketplace, "marketplace", "m", "US", "Marketplace to use")
	cmd.Execute()
}

func run(cmd *cobra.Command, args []string) {
	printWatermark()

	if marketplace == "" {
		cmd.Help()
		log.Fatal().Msg("marketplace is required")
	}

	marketplace := spapi.MarketplaceMap[marketplace]
	client := spapi.Client{
		ClientID:     viper.GetString("client_id"),
		ClientSecret: viper.GetString("client_secret"),
		Token: &oauth2.Token{
			RefreshToken: viper.GetString("refresh_token"),
		},

		SellerID: viper.GetString("seller_id"),
		HTTPClient: &http.Client{
			Timeout: 10 * time.Second,
		},
		Marketplace: &marketplace,
	}

	ctx := context.Background()

	from := time.Now().Add(-(30 * 24 * time.Hour))
	// amazon requires 5 days from order prior to requesting feedback.
	to := time.Now().Add(-(5 * 24 * time.Hour))

	opts := &spapi.GetOrdersRequest{
		LastUpdatedAfter:  from,
		LastUpdatedBefore: to,

		MarketplaceId: marketplace.ID,

		OrderStatuses: []string{"Shipped"},
	}

	// the limit outside of review requesting is 30 days.
	orders, err := client.GetOrders(ctx, opts)
	if err != nil {
		log.Fatal().Err(err).Msg("failed to get orders")
	}

	for _, order := range orders {
		if err := client.CreateProductReviewAndSellerFeedbackSolicitation(ctx, order.AmazonOrderId); err != nil {
			if v, ok := err.(spapi.Error); ok {
				if len(v.Errors) > 0 {
					err := v.Errors[0]

					switch err.Message {
					case "You can’t use this feature to request a review outside the 5-30 day range after the order delivery date.":
						continue
					case "You have already requested a review for this order.":
						continue
					default:
						log.Error().Str("error", v.Error()).Str("order_id", order.AmazonOrderId).Msg("failed to create feedback request")
						continue
					}
				}
			}

			// tbh, nothing really that we should do here because the next time this is run (within that 30 day time period)
			// we'll get the same order id and try again.
			log.Error().Err(err).Str("order_id", order.AmazonOrderId).Msg("failed to create feedback request")
			continue
		}

		log.Info().Str("order_id", order.AmazonOrderId).Msg("created feedback request")
	}
}

func printWatermark() {
	fmt.Println(`
	N   N  EEEE  RRRR  DDDD   W   W   AAA   RRRR  EEEE
	NN  N  E     R   R D   D  W   W  A   A  R   R E
	N N N  EEE   RRRR  D   D  W W W  AAAAA  RRRR  EEE
	N  NN  E     R  R  D   D  WW WW  A   A  R  R  E
	N   N  EEEE  R   R DDDD   W   W  A   A  R   R EEEE

	L     AAA   BBBB  SSSS
	L    A   A  B   B  S
	L    AAAAA  BBBB    SS
	L    A   A  B   B     S
	LLLL A   A  BBBB  SSSS
	`)
}
